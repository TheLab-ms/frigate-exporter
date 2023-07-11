# Frigate PostgreSQL Exporter

A tiny Go process to poll the [Frigate](https://github.com/blakeblackshear/frigate) API and write motion events to a postgres database.

At [thelab.ms](https://thelab.ms) we use it to track per-room utilization metrics i.e. how many hours per month a room is occupied.

## Usage

Builds are available as container images hosted by the Github registry.

Provide configuration in environment variables:

- `FRIGATE_URL`: Base URL of Frigate
- `CAMERAS`: Comma-delimited list of camera names to scrape
- `SCRAPE_INTERVAL`: How often to run
- `POSTGRES_HOST`, `POSTGRES_USER`, `POSTGRES_PASSWORD`: Postgres connection info
