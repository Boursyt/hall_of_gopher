# Hall of Gopher

## Overview
Hall of Gopher is a Go web app that lets users upload images, stores them in Google Cloud Storage, and shows a processed gallery. A Cloud Function resizes/crops images in the background.

Project notes
- This is a study project.
- The absence of Terraform IaC is intentional and part of the requirements.

Goals
- Provide a simple upload -> processing -> display flow.
- Demonstrate Go + GCS + Cloud Function + Cloud Run integration.

Tech stack
- Go (HTTP server, HTML templates)
- Google Cloud Storage (storage + signed URLs)
- Cloud Functions (resize/crop)
- Cloud Run (webapp deployment)
- Docker + Docker Compose (local dev/prod)

## Webapp
- Main code: `app/main.go`
- Routes:
  - `/home`: home page
  - `/upload`: upload form
  - `/api/images`: JSON list of images
  - `/qrcode`: QR code to `/home`
- Uploads to bucket `YOUR_GCS_BUCKET` under `img/before/`.
- Lists images from `img/after/` with signed URLs cached for 30 minutes.
- Templates and assets in `app/templates/` and `app/static/`.

### Run locally (dev)
```bash
docker compose up --build
```

### Run locally (prod)
```bash
docker compose -f docker-compose-prod.yml up --build
```

## Resizer
- Code: `resize_script/function.go`
- Trigger: GCS event on `img/before/`.
- Processing: center crop to 800x600, JPEG encoding (quality 85).
- Output: `img/after/`.

## Infra
- `Dockerfile`:
  - `build`: compiles the Go binary
  - `dev`: hot reload via `air`
  - `prod`: minimal image with binary + templates + static
- `docker-compose.yml`: image `cours_saas_project-web-dev`
- `docker-compose-prod.yml`: image `cours_saas_project-web-prod`
- `infra/build.yml`: Cloud Build to Artifact Registry
- `infra/service.yml`: Cloud Run service with `serviceAccountName: YOUR_SERVICE_ACCOUNT`
