# Session Service - Authentication System

A simple, clean authentication service built with Go, Gin, GORM, and JWT tokens.

## Features

- ✅ User registration and login
- ✅ JWT access and refresh tokens
- ✅ Google OAuth integration
- ✅ Token refresh functionality
- ✅ User logout
- ✅ Clean, simple codebase using Gin + GORM

## Project Structure

```
services/session-service/
├── internal/
│   ├── domain/           # Business logic and models
│   ├── handlers/         # HTTP request handlers (Gin)
│   ├── infrastructure/   # Database and external services
│   │   ├── database/     # Database connection
│   │   └── repository/   # Data access layer (GORM)
│   └── service/          # Token and OAuth services
├── pkg/
│   └── types/           # Shared types and DTOs
├── main.go              # Application entry point
├── go.mod               # Go dependencies
└── env.example          # Environment variables template
```

## Quick Start

1. **Install dependencies:**
   ```bash
   go mod tidy
   ```

2. **Set up environment variables:**
   ```bash
   cp env.example .env
   # Edit .env with your actual values
   ```

3. **Set up PostgreSQL database:**
   ```sql
   CREATE DATABASE session_service;
   ```

4. **Run the service:**
   ```bash
   go run main.go
   ```

## API Endpoints

### Authentication
- `POST /auth/register` - User registration
- `POST /auth/login` - User login
- `POST /auth/refresh` - Refresh access token
- `POST /auth/logout` - User logout
- `POST /auth/oauth` - OAuth login (Google)

### Health Check
- `GET /health` - Service health status

## Example Usage

### Register a new user
```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "username": "username",
    "full_name": "John Doe",
    "password": "password123"
  }'
```

### Login
```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

### Refresh token
```bash
curl -X POST http://localhost:8080/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "your_refresh_token_here"
  }'
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | Database host | localhost |
| `DB_PORT` | Database port | 5432 |
| `DB_USER` | Database user | postgres |
| `DB_PASSWORD` | Database password | password |
| `DB_NAME` | Database name | session_service |
| `JWT_ACCESS_SECRET` | JWT access token secret | your-access-secret-key |
| `JWT_REFRESH_SECRET` | JWT refresh token secret | your-refresh-secret-key |
| `GOOGLE_CLIENT_ID` | Google OAuth client ID | - |
| `GOOGLE_CLIENT_SECRET` | Google OAuth client secret | - |
| `GOOGLE_REDIRECT_URL` | Google OAuth redirect URL | - |
| `PORT` | Server port | 8080 |

## Database Schema

The service automatically creates these tables using GORM auto-migration:

### Users Table
- `id` (string, primary key)
- `email` (string, unique)
- `username` (string, unique)
- `full_name` (string)
- `password_hash` (string)
- `created_at` (timestamp)
- `updated_at` (timestamp)

### Sessions Table
- `id` (string, primary key)
- `user_id` (string, foreign key)
- `access_token` (string, unique)
- `refresh_token` (string, unique)
- `expires_at` (timestamp)
- `created_at` (timestamp)
- `updated_at` (timestamp)

## Security Features

- Password hashing with bcrypt
- JWT token validation
- Refresh token rotation
- OAuth state parameter validation
- Built-in CORS middleware

## Why This Approach?

- **Simple**: Uses Gin + GORM instead of complex raw SQL
- **Modern**: Gin provides excellent performance and middleware support
- **Clean**: Clear separation of concerns with clean handlers
- **Maintainable**: Easy to understand and modify
- **Production-ready**: Includes proper error handling and validation
- **Scalable**: Can easily add more OAuth providers or features

## Gin Benefits

- **Performance**: One of the fastest HTTP web frameworks
- **Middleware**: Built-in middleware support
- **Validation**: Automatic request validation
- **Error Handling**: Clean error handling patterns
- **Routing**: Intuitive routing with groups
- **JSON**: Built-in JSON binding and responses
