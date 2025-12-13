# HSM Client Integration with Web Explorer

## Architecture Overview

### Current State
- **Public Explorer**: Browse/search chains (no auth needed)
- **HSM Server**: Manages LMS keys, no authentication currently
- **HSM Client CLI**: Command-line tool for HSM operations

### Proposed Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   Web Explorer UI                        │
│  ┌──────────────────┐    ┌──────────────────────────┐  │
│  │  Public Section  │    │  Authenticated Section   │  │
│  │  - Browse chains │    │  - Generate keys         │  │
│  │  - Search        │    │  - Sign messages         │  │
│  │  - Statistics    │    │  - List my keys          │  │
│  └──────────────────┘    └──────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                          │
                          │ HTTP (with JWT tokens)
                          ▼
┌─────────────────────────────────────────────────────────┐
│              Explorer Backend (Go)                       │
│  ┌──────────────┐         ┌──────────────────────┐     │
│  │  Auth Module │         │  HSM Proxy           │     │
│  │  - Register  │         │  - Add user context  │     │
│  │  - Login     │         │  - Forward requests  │     │
│  │  - JWT tokens│         │  - Filter by user_id │     │
│  └──────────────┘         └──────────────────────┘     │
└─────────────────────────────────────────────────────────┘
                          │
                          │ HTTP
                          ▼
┌─────────────────────────────────────────────────────────┐
│                 HSM Server                               │
│  ┌──────────────────────────────────────────────────┐   │
│  │  Enhanced with user_id support                   │   │
│  │  - Keys stored with user_id                     │   │
│  │  - Operations filtered by user_id               │   │
│  │  - Validate JWT tokens                          │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## Design Decisions

### 1. Authentication Strategy
**Option A: JWT Tokens (Recommended)**
- Stateless authentication
- Explorer generates/validates JWTs
- HSM server validates JWTs on each request
- Tokens stored in localStorage (browser)
- Simple, scalable

**Option B: Session-based**
- Server-side session storage
- Requires session management
- More complex but more control

**Recommendation**: JWT tokens (simpler, stateless)

### 2. User Registration/Login
- Simple email/username + password
- Passwords hashed with bcrypt
- User data stored in BoltDB (new `users` bucket)
- Registration endpoint: `POST /api/auth/register`
- Login endpoint: `POST /api/auth/login`
- Returns JWT token on successful login

### 3. Key Ownership
- Each LMS key is associated with a `user_id`
- Users can only see/manage their own keys
- HSM server filters operations by `user_id` from JWT token
- Public explorer still shows all keys (for chain browsing)

### 4. UI Integration

#### Public Section (No Auth Required)
- Browse chains
- Search by key_id, hash
- View statistics
- Current functionality remains public

#### Authenticated Section (Requires Login)
- Login/Register buttons in header
- After login, show:
  - **My Keys** tab: List user's keys
  - **Generate Key** button
  - **Sign Message** form (key_id + message)
  - **View My Chain** (quick link to user's key chains)

### 5. Database Changes

**HSM Server Database** (`hsm-data/keys.db`):
- Add `user_id` field to `LMSKey` struct
- Modify `StoreKey` to accept `user_id`
- Modify `GetAllKeys` to filter by `user_id` (for authenticated requests)

**Explorer Database** (new `explorer/users.db`):
- Store user accounts:
  ```
  - id (primary key)
  - username (unique)
  - email (unique, optional)
  - password_hash (bcrypt)
  - created_at
  ```

## Implementation Plan

### Phase 1: Authentication Backend
1. Create `explorer/auth.go`:
   - User registration (hash passwords)
   - User login (validate, return JWT)
   - JWT token generation/validation
   - User database operations

2. Add authentication endpoints:
   - `POST /api/auth/register`
   - `POST /api/auth/login`
   - `GET /api/auth/me` (get current user from token)
   - `POST /api/auth/logout` (client-side, just clear token)

### Phase 2: HSM Server Enhancement
1. Modify `LMSKey` struct to include `UserID`:
   ```go
   type LMSKey struct {
       KeyID      string
       UserID     string  // NEW: Owner of the key
       Index      uint64
       // ... rest of fields
   }
   ```

2. Update HSM server endpoints to accept `user_id`:
   - `/generate_key`: Extract `user_id` from JWT, associate with key
   - `/list_keys`: Filter by `user_id` from JWT
   - `/sign`: Verify key belongs to user from JWT

3. Add JWT validation middleware to HSM server

### Phase 3: Explorer UI Integration
1. Add authentication UI:
   - Login modal/form
   - Register modal/form
   - User menu (show username, logout)
   - Token storage in localStorage

2. Add authenticated section:
   - "My Keys" tab/page
   - Generate key form
   - Sign message form
   - Link to view user's chains

3. API integration:
   - All HSM requests include JWT token in `Authorization` header
   - Handle authentication errors (401) gracefully

### Phase 4: Security Enhancements
1. Add rate limiting (prevent abuse)
2. Add CORS configuration
3. HTTPS support (for production)
4. Password strength requirements
5. Account lockout after failed login attempts

## API Changes

### New Explorer Endpoints
```
POST   /api/auth/register    - Register new user
POST   /api/auth/login       - Login, get JWT token
GET    /api/auth/me          - Get current user info
POST   /api/auth/logout      - Logout (client clears token)

GET    /api/my/keys          - List user's keys (authenticated)
POST   /api/my/generate      - Generate key for user (authenticated)
POST   /api/my/sign          - Sign with user's key (authenticated)
```

### Modified HSM Server Endpoints
All endpoints now require `Authorization: Bearer <jwt_token>` header:
```
POST   /generate_key    - Now requires auth, adds user_id
GET    /list_keys       - Now requires auth, filters by user_id
POST   /sign            - Now requires auth, verifies key ownership
```

## UI Flow

### Public User Flow
1. Opens explorer → sees public browsing interface
2. Can search, browse chains, view statistics
3. Sees "Login" button in header
4. Clicks Login → modal appears
5. Can register or login

### Authenticated User Flow
1. Logs in → JWT token stored in localStorage
2. Header shows username instead of "Login"
3. New "My Keys" tab appears
4. Can generate keys (automatically associated with user)
5. Can sign messages with their keys
6. Can view their key chains
7. Keys are private to the user

## Security Considerations

1. **JWT Secret**: Store in environment variable, different per deployment
2. **Password Hashing**: Use bcrypt with cost factor 10+
3. **Token Expiration**: JWT tokens expire after 24 hours (configurable)
4. **HTTPS**: Required for production (passwords in transit)
5. **Input Validation**: Sanitize all user inputs
6. **Key Isolation**: Users cannot access other users' keys
7. **Rate Limiting**: Prevent brute force attacks

## Migration Path

1. **Backward Compatibility**: 
   - Existing keys without `user_id` can be assigned to a "system" user
   - Or require migration script

2. **Gradual Rollout**:
   - Phase 1: Auth system (explorer only)
   - Phase 2: HSM server integration
   - Phase 3: UI integration
   - Phase 4: Security hardening

## Example User Experience

### Registering
1. User clicks "Register"
2. Fills form: username, email, password
3. Submits → account created
4. Automatically logged in
5. Sees "My Keys" tab

### Generating a Key
1. User clicks "Generate Key"
2. Optionally enters custom key_id
3. Clicks "Generate"
4. Key created and associated with user
5. Appears in "My Keys" list

### Signing
1. User selects key from dropdown
2. Enters message
3. Clicks "Sign"
4. Signature generated and committed to Raft
5. User can view the chain in explorer

## Questions to Consider

1. **Key Naming**: Should users be able to use any key_id, or should it be prefixed with username?
   - Option A: Free-form key_id (user responsibility)
   - Option B: Auto-prefix with username (e.g., `user123:my_key`)
   - **Recommendation**: Option A (more flexibility)

2. **Multi-user Keys**: Should we support shared keys?
   - For now: No (one user per key)
   - Future: Could add key sharing/permissions

3. **Key Limits**: Should there be limits on keys per user?
   - Option A: Unlimited
   - Option B: Limit (e.g., 100 keys per user)
   - **Recommendation**: Start with unlimited, add limits later if needed

4. **Public Keys**: Should public keys be visible to anyone?
   - Yes, for verification purposes
   - But private keys are strictly private

## Next Steps

1. Review and approve architecture
2. Implement Phase 1 (auth backend)
3. Test authentication flow
4. Implement Phase 2 (HSM server changes)
5. Implement Phase 3 (UI integration)
6. Test end-to-end flow
7. Deploy and monitor

