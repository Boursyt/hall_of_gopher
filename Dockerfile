# ---- Build ----
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY app/go.mod app/go.sum ./
RUN go mod download
COPY app/ .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /server .

# ---- Dev ----
FROM golang:1.25-alpine AS dev
RUN go install github.com/air-verse/air@latest
WORKDIR /app
COPY app/go.mod app/go.sum ./
RUN go mod download
COPY app/ .
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]

# ---- Prod ----
FROM scratch AS prod
COPY --from=build /server /server
COPY app/templates/ /templates/
COPY app/static/ /static/
EXPOSE 8080
ENTRYPOINT ["/server"]
