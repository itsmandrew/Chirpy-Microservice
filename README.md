# Chirpy API (practice Project)

A lightweight Go microservice for creating, listing, and fetching “chirps” (short user messages) backed by PostgreSQL.

## Features
- Create chirps with a message
- List all chirps
- Fetch a specific chirp by ID
- Simple RESTful API design


## Technologies Used
- Go (Golang)
- PostgreSQL

## Getting Started


### Prerequisites
- Go installed on your machine
- PostgreSQL installed and running
- Air for live reloading
- Goose for migrations
- sqlc for SQL generation


### Setup
1. Clone the repository:
   ```bash
   git clone

2. Run the following commands to set up the project:
   ```bash
   cd chirpy-api
   go mod tidy
   ```

3. Create a PostgreSQL database named `chirpy`:
   ```sql
   CREATE DATABASE chirpy;
   ```

4. Create a `.env` file in the root directory with the following content:
   ```env
    DB_URL=postgres://username:password@localhost:5432/chirpy?sslmode=disable
    PLATFORM="dev"
    ```

5. Run the migrations to set up the database schema:
    ```bash
    goose up
    ```

6. Start the application:
   ```bash
   air
   ```
   The application will be running at `http://localhost:8080`.

