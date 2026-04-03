

unit-go:
	cd server && go test ./...

integration-test:
	cd server && go test -tags=integration -race -v -count=1 ./...

unit-web:
	cd web && npm test

test-all: unit-go unit-web integration-test

fix:
	cd server && go fix ./...

run-service:
	cd server && go run .

run-frontend:
	cd web && npm run dev

build-frontend:
	cd web && npm ci && npm run build

build: build-frontend
	rm -rf server/internal/static/dist
	cp -r web/dist server/internal/static/dist
	cd server && go build -o writerace .

docker-build:
	docker build -f server/Dockerfile -t writerace .