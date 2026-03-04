sqlc:
	sqlc generate;

postgres:
	docker run --name postgres12 -e POSTGRES_PASSWORD=secret -e POSTGRES_USER=root -p 5432:5432  -d postgres:12-alpine;

createdb:
	docker exec -it postgres12 createdb --username=root --owner=root IM;

migrateup:
	migrate -path db/migration -database "postgres://root:secret@localhost:5432/IM?sslmode=disable" -verbose up;

migratedown:
	migrate -path db/migration -database "postgres://root:secret@localhost:5432/IM?sslmode=disable" -verbose down;

migratedown1:
	migrate -path db/migration -database "postgres://root:secret@localhost:5432/IM?sslmode=disable" -verbose down 1;

migrateup1:
	migrate -path db/migration -database "postgres://root:secret@localhost:5432/IM?sslmode=disable" -verbose up 1;

dropdb:
	docker exec -it postgres12 dropdb IM;

	