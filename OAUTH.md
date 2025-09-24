# OAuth Device Code Flow Implementation

This document describes the OAuth 2.0 Device Code Flow implementation in the Tailstream Agent, which provides a frictionless setup experience for users on headless servers, containers, and any environment without a web browser.

## Overview

The OAuth Device Code Flow (RFC 8628) allows users to authorize the Tailstream Agent by using a separate device with a web browser. This eliminates the need to manually copy tokens and stream IDs, providing a "magical" setup experience.

## User Experience

### Before OAuth
```bash
$ tailstream-agent
Enter your Tailstream Stream ID: stream_abc123...
Enter your Tailstream Access Token: ts_token_xyz...
✅ Configuration saved
```

### After OAuth
```bash
$ tailstream-agent
🚀 Tailstream Agent Setup

Visit: https://app.tailstream.io/device
Enter code: BDWP-HQPK

Waiting for authorization... ⏳
✅ Connected to Tailstream!

Your streams (2/5):
[1] nginx-prod (prod-web-01)
[2] app-logs (staging-api)
[3] → Create new stream

Select streams to configure (comma-separated): 1,3
Enter name for new stream: web-server-logs
✅ Created stream: web-server-logs (stream_id: abc123...)
✅ Configured 2 streams: nginx-prod, web-server-logs

🎉 Setup complete! Agent starting...
```

## Architecture

### Flow Diagram

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Tailstream     │    │  User's Device   │    │  Tailstream     │
│  Agent (Server) │    │  (Phone/Laptop)  │    │  Web App        │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                        │                        │
         │ 1. Request device code │                        │
         │ ─────────────────────────────────────────────► │
         │                        │                        │
         │ ◄─────────────────────────────────────────────│
         │    2. device_code +    │                        │
         │       user_code        │                        │
         │                        │                        │
         │ 3. Show user_code      │                        │
         │    and URL             │                        │
         │                        │                        │
         │                        │ 4. Visit URL, enter    │
         │                        │    user_code, login   │
         │                        │ ─────────────────────► │
         │                        │                        │
         │                        │ ◄─────────────────────│
         │                        │ 5. Authorize agent    │
         │                        │                        │
         │ 6. Poll for token      │                        │
         │ ─────────────────────────────────────────────► │
         │                        │                        │
         │ ◄─────────────────────────────────────────────│
         │ 7. access_token        │                        │
         │                        │                        │
         │ 8. Fetch streams & plan│                        │
         │ ─────────────────────────────────────────────► │
         │                        │                        │
         │ ◄─────────────────────────────────────────────│
         │ 9. streams + plan info │                        │
```

## Implementation Details

### Key Files

- **`oauth.go`**: Complete OAuth client implementation
- **`main.go`**: Integration point that calls `setupOAuth()` when setup is needed
- **`setup.go`**: Updated `needsSetup()` function with OAuth support
- **`config.go`**: Configuration structure supporting both OAuth and legacy setups

### OAuth Client (`oauth.go`)

#### Core Functions

**`setupOAuth()`**
- Main entry point for OAuth setup flow
- Orchestrates the entire process from device code request to configuration save

**`requestDeviceCode()`**
- Requests device code from `/api/oauth/device/code` endpoint
- Returns device code, user code, and verification URI

**`pollForToken()`**
- Polls `/api/oauth/device/token` endpoint every 2 seconds
- Handles `authorization_pending` responses
- 10-minute timeout for authorization

**`fetchUserData()`**
- Retrieves user's existing streams and plan information
- Makes parallel requests to `/api/user/streams` and `/api/user/plan`

**`selectStreams()`**
- Interactive UI for stream selection and creation
- Respects plan limits and shows upgrade prompts when needed
- Handles both existing streams and new stream creation

**`createNewStream()`**
- Creates new streams via `/api/streams` endpoint
- Validates stream names and handles API errors

### Configuration Integration

#### Setup Detection (`needsSetup()`)

The agent determines if setup is needed by checking:

1. **OAuth Configuration**: `key` + `streams` array populated
2. **Legacy Configuration**: `key` + `ship.stream_id` populated
3. **Environment Variable**: `TAILSTREAM_KEY` set

```go
func needsSetup() bool {
    cfg := loadConfig()

    // OAuth setup complete
    if cfg.Key != "" && len(cfg.Streams) > 0 {
        return false
    }

    // Legacy setup complete
    if cfg.Key != "" && cfg.Ship.StreamID != "" {
        return false
    }

    // Environment variable fallback
    if os.Getenv("TAILSTREAM_KEY") != "" {
        return false
    }

    return true
}
```

#### Configuration Generation

OAuth setup creates a complete configuration with:

```yaml
env: production
key: oauth_access_token_here
discovery:
  enabled: true
  paths:
    include:
      - "/var/log/nginx/*.log"
      - "/var/log/caddy/*.log"
      - "/var/log/apache2/*.log"
      - "/var/log/httpd/*.log"
      - "/var/www/**/storage/logs/*.log"
    exclude:
      - "**/*.gz"
      - "**/*.1"
updates:
  enabled: true
  channel: stable
  check_hours: 24
  max_delay_hours: 6
streams:
  - name: nginx-prod
    stream_id: stream_abc123
    paths:
      - "/var/log/nginx/*.log"
    exclude:
      - "**/*.gz"
  - name: web-server-logs
    stream_id: stream_xyz789
    paths:
      - "/var/log/nginx/*.log"
    exclude:
      - "**/*.gz"
```

## Backend Implementation Requirements

This section provides complete implementation guidance for the backend team to implement the OAuth Device Code Flow endpoints.

### Laravel Implementation

The following implementation uses Laravel with Passport/Sanctum for OAuth token management.

#### 1. Install Dependencies

```bash
# For OAuth token management
composer require laravel/sanctum
# OR for full OAuth 2.0 support
composer require laravel/passport

php artisan vendor:publish --provider="Laravel\Sanctum\SanctumServiceProvider"
php artisan migrate
```

#### 2. Database Migrations

Create migrations for device codes and enhanced stream management:

```php
// database/migrations/create_device_codes_table.php
public function up()
{
    Schema::create('device_codes', function (Blueprint $table) {
        $table->id();
        $table->string('device_code', 128)->unique();
        $table->string('user_code', 9)->unique();
        $table->string('client_id');
        $table->string('scope')->nullable();
        $table->boolean('authorized')->default(false);
        $table->unsignedBigInteger('user_id')->nullable();
        $table->timestamp('expires_at');
        $table->timestamps();

        $table->index(['user_code', 'expires_at']);
        $table->index(['device_code', 'expires_at']);
        $table->foreign('user_id')->references('id')->on('users');
    });
}

// database/migrations/add_oauth_fields_to_users_table.php
public function up()
{
    Schema::table('users', function (Blueprint $table) {
        $table->string('plan_name')->default('free');
        $table->json('plan_limits')->nullable();
    });
}

// database/migrations/enhance_streams_table.php
public function up()
{
    Schema::table('streams', function (Blueprint $table) {
        // Ensure these fields exist
        $table->string('stream_id')->unique();
        $table->string('name');
        $table->text('description')->nullable();
        $table->unsignedBigInteger('user_id');
        $table->timestamps();

        $table->foreign('user_id')->references('id')->on('users');
        $table->index(['user_id', 'created_at']);
    });
}
```

#### 3. Models

```php
// app/Models/DeviceCode.php
<?php
namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Carbon\Carbon;

class DeviceCode extends Model
{
    protected $fillable = [
        'device_code', 'user_code', 'client_id', 'scope',
        'authorized', 'user_id', 'expires_at'
    ];

    protected $casts = [
        'authorized' => 'boolean',
        'expires_at' => 'datetime',
    ];

    public function user()
    {
        return $this->belongsTo(User::class);
    }

    public function isExpired()
    {
        return Carbon::now()->isAfter($this->expires_at);
    }

    public static function generateUserCode()
    {
        $chars = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
        do {
            $code = '';
            for ($i = 0; $i < 4; $i++) {
                $code .= $chars[random_int(0, strlen($chars) - 1)];
            }
            $code .= '-';
            for ($i = 0; $i < 4; $i++) {
                $code .= $chars[random_int(0, strlen($chars) - 1)];
            }
        } while (self::where('user_code', $code)->exists());

        return $code;
    }
}

// app/Models/User.php (add methods)
public function streams()
{
    return $this->hasMany(Stream::class);
}

public function getPlanLimitsAttribute()
{
    $limits = [
        'free' => ['max_streams' => 3],
        'pro' => ['max_streams' => 10],
        'enterprise' => ['max_streams' => 50],
    ];

    return $limits[$this->plan_name] ?? $limits['free'];
}

public function canCreateStream()
{
    return $this->streams()->count() < $this->plan_limits['max_streams'];
}

// app/Models/Stream.php
<?php
namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Support\Str;

class Stream extends Model
{
    protected $fillable = ['name', 'stream_id', 'description', 'user_id'];

    public function user()
    {
        return $this->belongsTo(User::class);
    }

    protected static function boot()
    {
        parent::boot();
        static::creating(function ($stream) {
            if (!$stream->stream_id) {
                $stream->stream_id = Str::uuid();
            }
        });
    }
}
```

#### 4. Controllers

```php
// app/Http/Controllers/DeviceCodeController.php
<?php
namespace App\Http\Controllers;

use App\Models\DeviceCode;
use Illuminate\Http\Request;
use Illuminate\Support\Str;
use Carbon\Carbon;

class DeviceCodeController extends Controller
{
    public function requestCode(Request $request)
    {
        $request->validate([
            'client_id' => 'required|string',
            'scope' => 'nullable|string'
        ]);

        // Validate client_id
        if ($request->client_id !== 'tailstream-agent') {
            return response()->json(['error' => 'invalid_client'], 400);
        }

        $deviceCode = Str::random(40);
        $userCode = DeviceCode::generateUserCode();

        $deviceCodeRecord = DeviceCode::create([
            'device_code' => $deviceCode,
            'user_code' => $userCode,
            'client_id' => $request->client_id,
            'scope' => $request->scope ?? '',
            'expires_at' => Carbon::now()->addMinutes(10),
        ]);

        return response()->json([
            'device_code' => $deviceCode,
            'user_code' => $userCode,
            'verification_uri' => config('app.url') . '/device',
            'verification_uri_complete' => config('app.url') . '/device?user_code=' . $userCode,
            'expires_in' => 600,
            'interval' => 2
        ]);
    }

    public function exchangeToken(Request $request)
    {
        $request->validate([
            'grant_type' => 'required|in:urn:ietf:params:oauth:grant-type:device_code',
            'device_code' => 'required|string',
            'client_id' => 'required|string'
        ]);

        $deviceCodeRecord = DeviceCode::where('device_code', $request->device_code)
            ->where('client_id', $request->client_id)
            ->first();

        if (!$deviceCodeRecord) {
            return response()->json(['error' => 'invalid_grant'], 400);
        }

        if ($deviceCodeRecord->isExpired()) {
            $deviceCodeRecord->delete();
            return response()->json(['error' => 'expired_token'], 400);
        }

        if (!$deviceCodeRecord->authorized) {
            return response()->json(['error' => 'authorization_pending'], 400);
        }

        // Create access token using Sanctum
        $user = $deviceCodeRecord->user;
        $scopes = explode(' ', $deviceCodeRecord->scope);
        $token = $user->createToken('tailstream-agent', $scopes);

        // Clean up device code
        $deviceCodeRecord->delete();

        return response()->json([
            'access_token' => $token->plainTextToken,
            'token_type' => 'Bearer',
            'expires_in' => config('sanctum.expiration', 525600) * 60, // Default 1 year
            'scope' => $deviceCodeRecord->scope
        ]);
    }

    public function showAuthorizePage(Request $request)
    {
        $userCode = $request->input('user_code', '');
        return view('device-authorize', compact('userCode'));
    }

    public function authorizeDevice(Request $request)
    {
        $request->validate([
            'user_code' => 'required|string|size:9'
        ]);

        $userCode = strtoupper($request->user_code);
        $deviceCodeRecord = DeviceCode::where('user_code', $userCode)
            ->where('expires_at', '>', Carbon::now())
            ->first();

        if (!$deviceCodeRecord) {
            return back()->withErrors(['user_code' => 'Invalid or expired code']);
        }

        if ($deviceCodeRecord->authorized) {
            return back()->withErrors(['user_code' => 'Code already used']);
        }

        // Authorize the device
        $deviceCodeRecord->update([
            'authorized' => true,
            'user_id' => auth()->id(),
        ]);

        return view('device-success', ['deviceCode' => $deviceCodeRecord]);
    }
}

// app/Http/Controllers/StreamController.php
<?php
namespace App\Http\Controllers;

use App\Models\Stream;
use Illuminate\Http\Request;

class StreamController extends Controller
{
    public function __construct()
    {
        $this->middleware('auth:sanctum');
    }

    public function index()
    {
        $streams = auth()->user()->streams()
            ->select(['id', 'name', 'stream_id', 'description', 'created_at'])
            ->orderBy('created_at', 'desc')
            ->get();

        return response()->json(['streams' => $streams]);
    }

    public function store(Request $request)
    {
        $user = auth()->user();

        // Check plan limits
        if (!$user->canCreateStream()) {
            $maxStreams = $user->plan_limits['max_streams'];
            return response()->json([
                'error' => 'Stream limit exceeded',
                'message' => "Your {$user->plan_name} plan allows {$maxStreams} streams. Upgrade to create more."
            ], 403);
        }

        $request->validate([
            'name' => 'required|string|max:255',
            'description' => 'nullable|string|max:1000'
        ]);

        $stream = $user->streams()->create([
            'name' => $request->name,
            'description' => $request->description,
        ]);

        return response()->json($stream, 201);
    }
}

// app/Http/Controllers/UserController.php
<?php
namespace App\Http\Controllers;

class UserController extends Controller
{
    public function __construct()
    {
        $this->middleware('auth:sanctum');
    }

    public function plan()
    {
        $user = auth()->user();
        $currentStreams = $user->streams()->count();

        return response()->json([
            'plan_name' => ucfirst($user->plan_name),
            'limits' => $user->plan_limits,
            'current_usage' => [
                'streams' => $currentStreams
            ],
            'permissions' => [
                'can_create_streams' => $user->canCreateStream()
            ]
        ]);
    }
}
```

#### 5. Routes

```php
// routes/api.php
use App\Http\Controllers\DeviceCodeController;
use App\Http\Controllers\StreamController;
use App\Http\Controllers\UserController;

// OAuth Device Code Flow
Route::post('/oauth/device/code', [DeviceCodeController::class, 'requestCode']);
Route::post('/oauth/device/token', [DeviceCodeController::class, 'exchangeToken']);

// Protected API endpoints
Route::middleware('auth:sanctum')->group(function () {
    Route::get('/user/streams', [StreamController::class, 'index']);
    Route::get('/user/plan', [UserController::class, 'plan']);
    Route::post('/streams', [StreamController::class, 'store']);
});

// routes/web.php
Route::get('/device', [DeviceCodeController::class, 'showAuthorizePage'])
    ->name('device.authorize');
Route::post('/device', [DeviceCodeController::class, 'authorizeDevice'])
    ->middleware('auth')
    ->name('device.authorize.submit');
```

#### 6. Views

```blade
{{-- resources/views/device-authorize.blade.php --}}
@extends('layouts.app')

@section('title', 'Device Authorization')

@section('content')
<div class="max-w-md mx-auto mt-8 p-6 bg-white rounded-lg shadow-md">
    <div class="text-center mb-6">
        <h1 class="text-2xl font-bold text-gray-900">Device Authorization</h1>
        <p class="text-gray-600 mt-2">
            Authorize your Tailstream Agent to access your account
        </p>
    </div>

    <form method="POST" action="{{ route('device.authorize.submit') }}" class="space-y-4">
        @csrf

        <div>
            <label for="user_code" class="block text-sm font-medium text-gray-700 mb-2">
                Enter the code displayed on your device:
            </label>
            <input
                type="text"
                id="user_code"
                name="user_code"
                value="{{ $userCode }}"
                placeholder="XXXX-XXXX"
                maxlength="9"
                class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 text-center text-lg font-mono tracking-wider uppercase"
                required
            >
            @error('user_code')
                <p class="text-red-500 text-sm mt-1">{{ $message }}</p>
            @enderror
        </div>

        <button
            type="submit"
            class="w-full bg-blue-600 text-white py-2 px-4 rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition duration-200"
        >
            Authorize Device
        </button>
    </form>

    <div class="mt-6 text-center">
        <p class="text-sm text-gray-600">
            This will allow the Tailstream Agent to:
        </p>
        <ul class="text-sm text-gray-600 mt-2 space-y-1">
            <li>• Read your existing streams</li>
            <li>• Create new streams</li>
            <li>• Send log data to your streams</li>
        </ul>
    </div>
</div>

<script>
// Auto-format user code input
document.getElementById('user_code').addEventListener('input', function(e) {
    let value = e.target.value.replace(/[^A-Z0-9]/g, '');
    if (value.length > 4) {
        value = value.substring(0, 4) + '-' + value.substring(4, 8);
    }
    e.target.value = value;
});
</script>
@endsection

{{-- resources/views/device-success.blade.php --}}
@extends('layouts.app')

@section('title', 'Authorization Successful')

@section('content')
<div class="max-w-md mx-auto mt-8 p-6 bg-white rounded-lg shadow-md">
    <div class="text-center">
        <div class="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-4">
            <svg class="w-8 h-8 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
            </svg>
        </div>

        <h1 class="text-2xl font-bold text-gray-900 mb-2">Success!</h1>
        <p class="text-gray-600 mb-6">
            Your device has been authorized successfully. You can now return to your terminal.
        </p>

        <div class="bg-gray-50 p-4 rounded-md">
            <p class="text-sm text-gray-700">
                Code: <span class="font-mono font-bold">{{ $deviceCode->user_code }}</span>
            </p>
            <p class="text-sm text-gray-600 mt-1">
                Authorized at {{ $deviceCode->updated_at->format('H:i:s') }}
            </p>
        </div>
    </div>
</div>
@endsection
```

#### 7. Middleware & Security

```php
// app/Http/Middleware/ThrottleDeviceRequests.php
<?php
namespace App\Http\Middleware;

use Illuminate\Routing\Middleware\ThrottleRequests;

class ThrottleDeviceRequests extends ThrottleRequests
{
    protected function resolveRequestSignature($request)
    {
        // Throttle by IP + client_id for device code requests
        return sha1(
            $request->ip() . '|' . $request->input('client_id', '')
        );
    }
}

// In routes/api.php, add throttling:
Route::post('/oauth/device/code', [DeviceCodeController::class, 'requestCode'])
    ->middleware('throttle:5,1'); // 5 requests per minute per IP+client

Route::post('/oauth/device/token', [DeviceCodeController::class, 'exchangeToken'])
    ->middleware('throttle:30,1'); // 30 requests per minute (polling)
```

#### 8. Cleanup Job

```php
// app/Jobs/CleanupExpiredDeviceCodes.php
<?php
namespace App\Jobs;

use App\Models\DeviceCode;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;
use Carbon\Carbon;

class CleanupExpiredDeviceCodes implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    public function handle()
    {
        DeviceCode::where('expires_at', '<', Carbon::now())->delete();
    }
}

// Schedule in app/Console/Kernel.php
protected function schedule(Schedule $schedule)
{
    $schedule->job(CleanupExpiredDeviceCodes::class)->everyMinute();
}
```

### Testing the Backend

```bash
# Test device code request
curl -X POST http://localhost:8000/api/oauth/device/code \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=tailstream-agent&scope=stream:read stream:write stream:create"

# Test token exchange (will return authorization_pending initially)
curl -X POST http://localhost:8000/api/oauth/device/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=urn:ietf:params:oauth:grant-type:device_code&device_code=DEVICE_CODE&client_id=tailstream-agent"

# Test protected endpoints
curl -H "Authorization: Bearer ACCESS_TOKEN" \
  http://localhost:8000/api/user/streams
```

## API Endpoints

The OAuth implementation expects the following backend endpoints:

### Device Code Request
```
POST /api/oauth/device/code
Content-Type: application/x-www-form-urlencoded

client_id=tailstream-agent&scope=stream:read stream:write stream:create
```

**Response:**
```json
{
  "device_code": "device_code_here",
  "user_code": "BDWP-HQPK",
  "verification_uri": "https://app.tailstream.io/device",
  "verification_uri_complete": "https://app.tailstream.io/device?user_code=BDWP-HQPK",
  "expires_in": 600,
  "interval": 2
}
```

### Token Exchange
```
POST /api/oauth/device/token
Content-Type: application/x-www-form-urlencoded

grant_type=urn:ietf:params:oauth:grant-type:device_code&device_code=device_code_here&client_id=tailstream-agent
```

**Response (Pending):**
```json
{
  "error": "authorization_pending"
}
```

**Response (Success):**
```json
{
  "access_token": "access_token_here",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "stream:read stream:write stream:create"
}
```

### User Streams
```
GET /api/user/streams
Authorization: Bearer access_token_here
```

**Response:**
```json
{
  "streams": [
    {
      "id": 1,
      "name": "nginx-prod",
      "stream_id": "stream_abc123",
      "description": "Production nginx logs"
    }
  ]
}
```

### User Plan
```
GET /api/user/plan
Authorization: Bearer access_token_here
```

**Response:**
```json
{
  "plan_name": "Pro",
  "limits": {
    "max_streams": 10
  },
  "current_usage": {
    "streams": 3
  },
  "permissions": {
    "can_create_streams": true
  }
}
```

### Create Stream
```
POST /api/streams
Authorization: Bearer access_token_here
Content-Type: application/json

{
  "name": "web-server-logs",
  "description": "Web server access logs"
}
```

**Response:**
```json
{
  "id": 2,
  "name": "web-server-logs",
  "stream_id": "stream_xyz789",
  "description": "Web server access logs"
}
```

## Security Considerations

### Token Storage
- OAuth tokens are stored in the same configuration file as legacy tokens
- Configuration files are created with `0600` permissions (owner read/write only)
- Tokens are treated as Bearer tokens in Authorization headers

### Device Code Security
- Device codes expire after 10 minutes
- User codes are 8-character human-readable format (avoiding confusing characters)
- Polling interval respects server-specified interval (2 seconds)

### Network Security
- All requests use HTTPS
- 10-second HTTP timeout prevents hanging connections
- Proper error handling for network failures

## Backward Compatibility

The OAuth implementation maintains full backward compatibility:

### Existing Configurations
- Legacy `ship.url` and `ship.stream_id` configurations continue to work
- Environment variable `TAILSTREAM_KEY` continues to work
- No migration required for existing installations

### Mixed Environments
- Some agents can use OAuth while others use legacy configuration
- Gradual migration path available
- No breaking changes to existing functionality

## Error Handling

### Network Errors
```
Setup failed: failed to request device code: connection timeout
```

### Authorization Errors
```
Setup failed: authorization failed: oauth error: access_denied
```

### API Errors
```
Setup failed: failed to fetch user data: 401 Unauthorized
```

### Plan Limit Errors
```
Failed to create stream: Stream limit exceeded - Your plan allows 5 streams. Upgrade to create more.
```

## Testing

### Unit Tests
The implementation includes comprehensive tests in `setup_test.go`:

- Valid OAuth configuration detection
- Valid legacy configuration detection
- Environment variable fallback
- Incomplete configuration rejection
- Configuration with key but no streams

### Integration Testing
```bash
# Test OAuth setup flow (will show 404 until backend is ready)
DEBUG=1 timeout 5s ./tailstream-agent

# Test with existing OAuth config
echo "key: test\nstreams:\n- name: test\n  stream_id: test123\n  paths: ['/var/log/*.log']" > tailstream.yaml
./tailstream-agent

# Test with legacy config
echo "key: test\nship:\n  url: https://example.com\n  stream_id: test123" > tailstream.yaml
./tailstream-agent

# Test with environment variable
TAILSTREAM_KEY=test ./tailstream-agent
```

## Deployment Considerations

### Backend Requirements
- OAuth endpoints must be implemented before enabling OAuth flow
- Device authorization page at `/device` must be created
- User authentication system must be integrated

### Rollout Strategy
1. Deploy backend OAuth endpoints
2. Test OAuth flow in staging environment
3. Gradually enable OAuth for new installations
4. Existing installations continue working with legacy flow

### Monitoring
- Track OAuth success/failure rates
- Monitor device code expiration rates
- Alert on unusual authorization patterns

## Future Enhancements

### Token Refresh
- Implement refresh token support for long-lived installations
- Automatic token renewal before expiration

### Enhanced Security
- Certificate pinning for API endpoints
- Device fingerprinting for additional security

### User Experience
- QR code generation for mobile authorization
- Progress indicators during polling
- Better error messages with recovery suggestions

This implementation provides a solid foundation for frictionless agent setup while maintaining security and backward compatibility.