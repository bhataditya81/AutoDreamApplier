# Infrastructure & Data Layer Architecture

AutoDreamApplier is supported by a remarkably dense infrastructure topology designed to gracefully handle connection pools, raw documents, and massively disparate messaging speeds.

## 1. Core Data Nodes

### 1.1 Relational Persistence (PostgreSQL 16)
The primary system of record is a `pgvector`-enabled Postgres deployment.
- **Why PG 16?**: JSONB optimizations required for dynamic User Preferences and the native capability to run `IVFFlat` indexes on dense semantic embeddings (future-proofing the semantic Job Matcher).
- **Migration Engine**: Versioned schema updates are automatically applied on container startup via `golang-migrate`, assuring robust deployment topologies without manual SQL drops.

### 1.2 The Message Bus & Cache (Redis 7)
Redis sits between the API Gateway and the Apply Engine.
- **Asynq Operations**: Redis physically stores the `ai_prep` and `browser_apply` task queues. It tracks retry policies, failure thresholds, and backoff exponential delays natively.
- **Rate Limit Caching**: The API Gateway may offload rapid-fire IP checks into a Redis memory cache to prevent abuse.

### 1.3 Object Storage (MinIO / AWS S3)
While database persistence saves structured data, MinIO acts as an S3-compatible local gateway for Blobs:
- **`autodream-resumes`**: The initial raw user resumes alongside dynamically generated PDF counterparts parsed from the AI tailor engine.
- **`autodream-screenshots`**: Every successful Browser Pool interaction fires off a concluding screenshot. Uploading these transparent images proves to the user that progress was completed by the ATS.

## 2. Telemetry and System Observability 

Because automation fails silently, Observability is considered a Tier-1 implementation priority within the `docker-compose.yml`.

### 2.1 Prometheus Scraping
Prometheus targets standard `/metrics` endpoints.
- Scrapes the `Go` runtimes (Memory allocations, Goroutine deadlocks).
- Scrapes a `redis-exporter` sidecar image natively analyzing Asynq queue depths to track if the system is backlogging. 

### 2.2 Grafana Dashboards
A pre-provisioned Grafana configuration visualizes the timeseries data queried from Prometheus. 
- Allows operators to distinctly see if the Job Discovery service is throwing `HTTP 403 / 429` rate limiting graphs from Indeed/Glassdoor.
- Visualizes CPU load on the memory-thirsty `browser-pool` instances protecting against Out of Memory (OOM) fatal kills.
