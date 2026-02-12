# ---- Base ----
FROM python:3.13-slim AS base
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# ---- Dev ----
FROM base AS dev
ENV FLASK_ENV=development
COPY . .
CMD ["flask", "run", "--host=0.0.0.0", "--port=8080", "--debug"]

# ---- Prod ----
FROM base AS prod
COPY . .
CMD ["gunicorn", "--bind", "0.0.0.0:8080", "app:app"]
