# Services Dashboard

A modern Go web application to monitor and display all ~110 microservices running on the 0crawl infrastructure.

## Features

- 📊 **Real-time Health Monitoring** - Auto-refreshes every 30 seconds
- 🔍 **Search & Filter** - Find services by name, description, or tags
- 📁 **Category Grouping** - domains, security, recon, infrastructure, web_analysis
- 🎨 **Modern Dark Theme** - Glassmorphism effects, responsive design
- ⚡ **Fast** - Single Go binary with embedded frontend

## Quick Start

```bash
# Run locally
go run .

# Build Docker image
docker build -t go_services_dashboard .

# Run with Docker
docker run -p 8080:8080 go_services_dashboard
```

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /api/services` | List all services (supports `?category=`, `?status=`, `?q=` filters) |
| `GET /api/services/:id` | Get single service details |
| `GET /api/categories` | List categories with counts |
| `GET /api/stats` | Aggregate health statistics |
| `GET /health` | Dashboard health check |
| `GET /version` | Dashboard version info |

## Deployment

```bash
# Push to GHCR
docker push ghcr.io/baditaflorin/go_services_dashboard:latest

# On server
docker pull ghcr.io/baditaflorin/go_services_dashboard:latest
docker-compose up -d
```

## License

MIT
