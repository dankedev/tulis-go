# Tulis Backend API

A Go-based REST API backend for Tulis, a content management system built with Fiber framework.

## Tech Stack

- **Framework**: [Go Fiber](https://gofiber.io/) v2
- **ORM**: [GORM](https://gorm.io/) (MySQL & SQLite support)
- **Authentication**: JWT with [golang-jwt/jwt](https://github.com/golang-jwt/jwt)
- **Storage**: Local filesystem or Cloudflare R2 (S3-compatible)
- **Documentation**: Swagger/OpenAPI via [swaggo](https://github.com/swaggo/swag)
- **Logging**: [Logrus](https://github.com/sirupsen/logrus)
- **Container**: Docker & Docker Compose

## Project Structure

```
backend/
├── cmd/api/              # Application entry point
│   └── main.go
├── config/               # Configuration loading
├── domain/               # Domain-driven architecture
│   ├── importer/         # WordPress WXR import functionality
│   ├── media/            # Media management
│   ├── plugin/           # Plugin system
│   ├── post/             # Posts, taxonomies, post types
│   ├── user/             # User authentication & management
│   └── workspace/        # Multi-tenant workspace management
├── middleware/            # Fiber middleware (auth, tenant scoping)
├── routes/               # Route registration
├── storage/              # Storage abstraction (local/R2)
├── utils/                # Utilities (JWT service)
├── docs/                 # Swagger documentation
├── docker-compose.yml    # Docker services
├── Dockerfile
└── .env.example          # Environment template
```

## Getting Started

### Prerequisites

- Go 1.21+
- MySQL 8.0+ or SQLite
- Docker & Docker Compose (optional)

### Local Development

1. **Clone and install dependencies**

```bash
cd tulis-go
go mod download
```

2. **Setup environment variables**

```bash
cp .env.example .env
# Edit .env with your database credentials and secrets
```

3. **Configure database**

Edit `.env` with your database settings:

```env
APP_ENV=development
APP_PORT=8080
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your_password
DB_NAME=tulis_app
JWT_SECRET=your-secret-key
```

4. **Run with live reload**

```bash
# Using Air for live reload
air
```

Or directly:

```bash
go run cmd/api/main.go
```

5. **Access Swagger documentation**

Open [http://localhost:8080/swagger](http://localhost:8080/swagger) after starting the server.

### Docker Development

```bash
# Build and run all services
docker-compose up --build

# Run only the API container
docker-compose up --build api
```

The API will be available at `http://localhost:8080`.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_ENV` | Environment mode | `development` |
| `APP_PORT` | Server port | `8080` |
| `DB_HOST` | Database host | `127.0.0.1` |
| `DB_PORT` | Database port | `3306` |
| `DB_USER` | Database user | `root` |
| `DB_PASSWORD` | Database password | - |
| `DB_NAME` | Database name | `tulis_app` |
| `JWT_SECRET` | JWT signing secret | `super-secret-key` |
| `JWT_EXPIRY_HOURS` | Token expiry in hours | `24` |
| `WORKSPACE_RESTRICTED` | Restrict workspace creation | `false` |
| `ALLOW_REGISTRATION` | Allow new user signup | `true` |
| `R2_ACCOUNT_ID` | Cloudflare R2 account ID | - |
| `R2_ACCESS_KEY_ID` | R2 access key | - |
| `R2_SECRET_ACCESS_KEY` | R2 secret key | - |
| `R2_BUCKET_NAME` | R2 bucket name | `tulis-media` |
| `R2_PUBLIC_URL` | R2 public URL | - |

## API Structure

### Public Endpoints (Rate Limited)

```
GET  /api/v1/public/posts           # List published posts
GET  /api/v1/public/posts/:slug     # Get post by slug
GET  /api/v1/public/media/:id       # Get media by ID
```

### Authentication Endpoints

```
POST /api/user/register              # User registration
POST /api/user/login                 # User login
GET  /api/user/me                    # Get current user
```

### Protected Endpoints (Require JWT)

```
# Workspace
GET    /api/workspaces                # List user's workspaces
POST   /api/workspaces                # Create workspace
GET    /api/workspaces/:id            # Get workspace
PUT    /api/workspaces/:id            # Update workspace
DELETE /api/workspaces/:id            # Delete workspace
POST   /api/workspaces/:id/members    # Add member
DELETE /api/workspaces/:id/members/:userId  # Remove member

# Posts
GET    /api/posts                     # List posts (tenant-scoped)
POST   /api/posts                     # Create post
GET    /api/posts/:id                 # Get post
PUT    /api/posts/:id                 # Update post
DELETE /api/posts/:id                 # Delete post
POST   /api/posts/:id/revisions       # Create revision

# Taxonomies
GET    /api/taxonomies                # List taxonomies
POST   /api/taxonomies                # Create taxonomy
PUT    /api/taxonomies/:id            # Update taxonomy
DELETE /api/taxonomies/:id            # Delete taxonomy

# Media
GET    /api/media                     # List media
POST   /api/media/upload              # Upload media
DELETE /api/media/:id                 # Delete media

# Plugins
GET    /api/plugins                   # List plugins
POST   /api/plugins                   # Install plugin
PUT    /api/plugins/:id               # Update plugin
DELETE /api/plugins/:id               # Uninstall plugin

# Importer
POST   /api/import/wxr                # Import WordPress WXR
GET    /api/import/logs               # Import logs
```

### Headers

Protected endpoints require:

```
Authorization: Bearer <jwt_token>
X-Workspace-ID: <workspace_id>
```

## Features

### Multi-tenant Workspaces

- Each workspace operates as an independent tenant
- Members can be assigned roles within workspace
- All data is scoped to workspace context

### Post Management

- Custom post types
- Taxonomy support (categories, tags)
- Post revisions for content history
- Markdown support

### Media Library

- File upload with automatic optimization
- Cloudflare R2 or local storage
- Image processing support

### WordPress Importer

- Import posts from WordPress WXR format
- Preserves media attachments
- Migration tool for content transfer

### Plugin System

- Extensible workspace plugins
- Enable/disable plugins per workspace

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific domain tests
go test ./domain/post/...
```

## Development

### Generate Swagger Documentation

```bash
swag init -g cmd/api/main.go -o docs
```

### Code Formatting

```bash
go fmt ./...
go mod tidy
```

## License

Private project. All rights reserved.
