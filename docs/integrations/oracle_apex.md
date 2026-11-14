# Oracle APEX Single Sign-On (SSO) Integration with ZITADEL

This guide provides step-by-step instructions for integrating **Oracle Application Express (APEX)** (running on Oracle Database 26ai / 23ai or OCI Autonomous Database) with **ZITADEL** as the centralized OpenID Connect (OIDC) Identity Provider.

---

## 1. Architecture Overview

Oracle APEX applications operate as **OpenID Connect Relying Parties (Clients)**, delegating all authentication, multi-factor authentication (MFA/Passkeys), and user lifecycle management to ZITADEL.

```
+-------------------+              1. Login Redirect              +--------------------+
|                   | ──────────────────────────────────────────> |                    |
|    Oracle APEX    |                                             |  ZITADEL Instance  |
|  (Relying Party)  | <────────────────────────────────────────── |  (OpenID Provider) |
|                   |          2. Auth Code via ORDS Callback     |                    |
+-------------------+                                             +--------------------+
          │                                                                 ▲
          │ 3. Exchange Code for ID & Access Token (Backchannel)            │
          └─────────────────────────────────────────────────────────────────┘
          │
          ▼ 4. Post-Authentication Hook: Map ZITADEL Roles to APEX Authorization Schemes
```

### Benefits
* **Universal SSO**: Single Sign-On across Oracle APEX low-code applications and enterprise microservices.
* **Modern Passwordless & MFA**: Enable Passkeys (FIDO2/WebAuthn), TOTP, and SMS MFA for APEX users without writing custom PL/SQL auth code.
* **Multi-Tenancy**: Isolate APEX applications and users by ZITADEL Organizations.
* **Role-Based Access Control (RBAC)**: Centralize application permissions in ZITADEL and automatically map them to APEX Authorization Schemes.

---

## 2. Step 1: Create an Application in ZITADEL

1. Log in to the **ZITADEL Console**.
2. Select your **Organization** and open your target **Project** (or create a new Project named `Oracle APEX Apps`).
3. Click **New Application**:
   * **Name**: `Oracle APEX Web App`
   * **Type**: `Web Application`
   * **Authentication Method**: `Code` (Authorization Code with PKCE)
4. Configure Redirect URIs:
   * **Redirect URI**:
     ```text
     https://<your-apex-host>/ords/apex_authentication.callback
     ```
     *(Replace `<your-apex-host>` with your Oracle REST Data Services (ORDS) domain)*.
   * **Post Logout Redirect URI**:
     ```text
     https://<your-apex-host>/ords/r/<workspace>/<app_alias_or_id>/login
     ```
5. Note the generated **Client ID** and **Client Secret**.
6. In **Application Settings -> Token Settings**:
   * Check **Assert Roles on Authentication**.
   * Check **User Info inside ID Token**.
   * Check **Add User roles into ID token**.

---

## 3. Step 2: Configure Oracle Database Network ACL (Host ACE) & TLS Trust

> [!IMPORTANT]
> **Database Outbound Network Requirement**
> When Oracle APEX performs OpenID Connect Discovery (`/.well-known/openid-configuration`) and exchanges the Authorization Code for tokens (`/oauth/v2/token`), the Oracle Database executes outbound HTTPS calls via `UTL_HTTP`.
> By default, Oracle Database blocks all outbound socket connections. Failure to grant network privileges will cause login to fail with **`ORA-24247: network access denied by access control list (ACL)`**.

### 1. Grant Network Access Control (Host ACE)
Connect as `ADMIN` or `SYS` (or `PDB_ADMIN`) to your Oracle database schema and grant outbound HTTP/HTTPS privileges to the APEX engine user (e.g. `APEX_230200`, `APEX_240100`, or your workspace schema):

```sql
BEGIN
    -- Grant outbound connect and resolve privileges to your ZITADEL host
    DBMS_NETWORK_ACL_ADMIN.APPEND_HOST_ACE(
        host       => '<your-zitadel-domain>', -- e.g. 'auth.example.com' or '*'
        lower_port => 443,
        upper_port => 443,
        ace        => xs$ace_type(
            privilege_list => xs$name_list('http', 'http_proxy'),
            principal_name => 'APEX_240100', -- Replace with your APEX database schema user
            principal_type => xs_acl.ptype_db
        )
    );
END;
/
COMMIT;
```

*(For OCI Autonomous Database / ATP, the `APEX_PUBLIC_USER` and APEX schemas typically have default internet access for verified public domains, but private or custom endpoint ZITADEL instances require the above ACE).*

### 2. Configure Oracle Wallet for Custom TLS / Internal Certificates (If Applicable)
If your ZITADEL instance uses an internal, corporate, or self-signed CA certificate (rather than a well-known public CA trusted by OCI/Oracle defaults):
1. Export the ZITADEL Root CA certificate in PEM format.
2. Import the root CA into the Oracle Database Wallet via `orapki`:
   ```bash
   orapki wallet add -wallet /opt/oracle/wallet -trusted_cert -cert zitadel_ca.pem -pwd WalletPassword123
   ```
3. In **APEX Administration Services -> Manage Instance -> Security -> Wallet**:
   * Set **Wallet Path**: `file:/opt/oracle/wallet`
   * Set **Wallet Password**: `<WalletPassword>`

---

## 4. Step 3: Configure Web Credentials in Oracle APEX

1. Log in to your **APEX Workspace** as a Developer / Administrator.
2. Navigate to **App Builder** -> select your application -> **Shared Components**.
3. Under **Security**, click **Web Credentials**.
4. Click **Create**:
   * **Credential Name**: `ZITADEL_OIDC_CREDENTIAL`
   * **Static Identifier**: `ZITADEL_OIDC_CREDENTIAL`
   * **Authentication Type**: `Basic Authentication` / `OAuth2 Client Credentials`
   * **Client ID / Username**: `<ZITADEL_CLIENT_ID>`
   * **Client Secret / Password**: `<ZITADEL_CLIENT_SECRET>`
5. Click **Apply Changes**.

---

## 5. Step 4: Create OpenID Connect Authentication Scheme

1. In **Shared Components**, click **Authentication Schemes**.
2. Click **Create** -> select **Based on a pre-configured scheme from the gallery** or **Custom**.
3. Configure the scheme:
   * **Name**: `ZITADEL OpenID Connect`
   * **Scheme Type**: `Social Sign-In` / `OpenID Connect Provider`
   * **Credential Store**: `ZITADEL_OIDC_CREDENTIAL`
   * **Discovery URL**:
     ```text
     https://<your-zitadel-domain>/.well-known/openid-configuration
     ```
   * **Scope**:
     ```text
     openid profile email urn:zitadel:iam:org:project:roles
     ```
   * **Username Attribute**: `preferred_username` (or `email`)
4. Save and set this scheme as the **Current Scheme**.

---

## 6. Step 5: Map ZITADEL Roles to APEX Authorization Schemes

ZITADEL includes the user's project roles in the ID token claim `urn:zitadel:iam:org:project:roles`. You can automatically synchronize these roles to APEX session authorization via a Post-Authentication procedure.

### 1. Create a Post-Authentication PL/SQL Procedure
In your application's database schema, create the following package or standalone procedure:

```sql
CREATE OR REPLACE PROCEDURE sync_zitadel_user_roles IS
    l_json_text CLOB;
    l_roles     APEX_T_VARCHAR2;
BEGIN
    -- Read ID token or userinfo claims retrieved by APEX OIDC
    l_json_text := apex_json.get_clob_output;
    
    -- Parse claims into APEX session state
    -- APEX stores OAuth/OIDC attributes in apex_custom_auth or session state
    APEX_CUSTOM_AUTH.SET_USER(p_user => LOWER(v('APP_USER')));
    
    -- Example: Set application session item based on ZITADEL role claims
    -- In APEX 23+/24+/26ai, APEX_AUTHENTICATION.GET_CALLBACK_DATA returns token attributes:
    -- Check if user possesses the 'admin' role in ZITADEL:
    IF APEX_JSON.GET_BOOLEAN(p_path => 'urn:zitadel:iam:org:project:roles.admin') THEN
        APEX_UTIL.SET_SESSION_STATE('P0_USER_ROLE', 'ADMIN');
    ELSE
        APEX_UTIL.SET_SESSION_STATE('P0_USER_ROLE', 'USER');
    END IF;
END sync_zitadel_user_roles;
/
```

### 2. Attach Procedure to APEX Authentication Scheme
1. Go to **Shared Components** -> **Authentication Schemes** -> **ZITADEL OpenID Connect**.
2. Scroll to **Login Processing** -> **Post-Authentication Procedure Name**.
3. Enter: `sync_zitadel_user_roles`.
4. Click **Apply Changes**.

### 3. Define APEX Authorization Schemes
1. Go to **Shared Components** -> **Authorization Schemes**.
2. Create scheme `IS_ADMIN`:
   * **Scheme Type**: `PL/SQL Function Returning Boolean`
   * **PL/SQL Expression**:
     ```sql
     RETURN :P0_USER_ROLE = 'ADMIN';
     ```
3. Attach `IS_ADMIN` to sensitive pages, interactive grids, or navigation menu items.

---

## 7. Step 6: Configure Single Sign-Out (SLO)

When a user clicks "Sign Out" in APEX, terminate both the local APEX session and the ZITADEL session:

1. In **Shared Components** -> **Authentication Schemes** -> **ZITADEL OpenID Connect**.
2. Under **Post-Logout**, select **Go to URL**:
   ```text
   https://<your-zitadel-domain>/oidc/v1/end_session?post_logout_redirect_uri=https%3A%2F%2F<your-apex-host>%2Fords%2Fr%2F<workspace>%2F<app_id>%2Flogin
   ```
3. When the user logs out from APEX, they are cleanly logged out of ZITADEL and redirected back to the APEX login splash screen.

---

## 8. Step 7: Verification Checklist

| Step | Check Item | Expected Result |
| :--- | :--- | :--- |
| 1 | Navigate to APEX App URL | Automatically redirected to ZITADEL hosted login page |
| 2 | Authenticate in ZITADEL (Passkey / Password + MFA) | Successfully authenticated and redirected back to APEX dashboard |
| 3 | Inspect APEX Session State | `APP_USER` matches ZITADEL `preferred_username` or `email` |
| 4 | RBAC Check | User with `admin` role can access admin pages; standard user is restricted |
| 5 | Sign Out | Session terminated in both APEX and ZITADEL |
