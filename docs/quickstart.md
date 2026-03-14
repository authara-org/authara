# Quickstart

The fastest way to run **Authara** locally is with Docker.

This quickstart uses:

- **Authara migrations** – applies the required database schema
- **Authara** – the authentication service
- **Authara Gateway** – the reverse proxy
- your application running locally on port `8081`

---

## 1. Create configuration file

Create a file called `.env`:

```bash
touch .env
```

Example minimal configuration:

```env
APP_ENV=dev

POSTGRESQL_HOST=host.docker.internal
POSTGRESQL_PORT=5432
POSTGRESQL_DATABASE=authara
POSTGRESQL_USERNAME=authara
POSTGRESQL_PASSWORD=authara

PUBLIC_URL=http://localhost:3000

AUTHARA_JWT_ISSUER=authara
AUTHARA_JWT_ACTIVE_KEY_ID=key-1
AUTHARA_JWT_KEYS=key-1:VZp2u1sYz0g2nF2vY8q8dP7cZQpL5cRrXn0k7FZ0xkE=
```

For the full list of available variables, see the configuration reference.

---

## 2. Run database migrations

Authara requires a PostgreSQL database with the correct schema.

Run the migrations container:

```bash
docker run --rm \
  --env-file .env \
  ghcr.io/authara-org/authara-migrations:latest
```

This applies the required database schema.

---

## 3. Start Authara

```bash
docker run -d \
  --name authara \
  --env-file .env \
  -p 8080:8080 \
  ghcr.io/authara-org/authara-core:latest
```

---

## 4. Start the gateway

```bash
docker run -d \
  -p 3000:3000 \
  -e GATEWAY_BIND=:3000 \
  -e AUTHARA_UPSTREAM=host.docker.internal:8080 \
  -e APP_UPSTREAM=host.docker.internal:8081 \
  ghcr.io/authara-org/authara-gateway:latest
```

This setup assumes:

- a PostgreSQL instance is running on `localhost:5432`
- your application is reachable on `localhost:8081`

The gateway connects to services running on the host via `host.docker.internal`.

`host.docker.internal` is supported by Docker Desktop and allows containers
to reach services running on the host machine.

---

## 5. Open the login page

Open:

```
http://localhost:3000/auth/login
```

If everything is running correctly, you should see the Authara login page.

---

## Docker Compose

Authara can be added to an existing Docker Compose stack.

Below is a minimal example including:

- a PostgreSQL database
- a simple example application
- the Authara migrations container
- Authara
- the Authara Gateway

```yaml
services:

  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: authara
      POSTGRES_USER: authara
      POSTGRES_PASSWORD: authara

  app:
    image: nginx:alpine

  authara-migrations:
    image: ghcr.io/authara-org/authara-migrations:latest
    env_file:
      - .env
    depends_on:
      - postgres

  authara:
    image: ghcr.io/authara-org/authara-core:latest
    env_file:
      - .env
    depends_on:
      authara-migrations:
        condition: service_completed_successfully

  gateway:
    image: ghcr.io/authara-org/authara-gateway:latest
    ports:
      - "3000:3000"
    environment:
      GATEWAY_BIND: :3000
      AUTHARA_UPSTREAM: authara:8080
      APP_UPSTREAM: app:80
    depends_on:
      - authara
      - app
```

Create a `.env` file containing the Authara configuration.

Start the stack:

```bash
docker compose up
```

Then open:

```
http://localhost:3000/auth/login
```

---

## Next steps

- Configuration overview
- Architecture overview
- Deployment guide
