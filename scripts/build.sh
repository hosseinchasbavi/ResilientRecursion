# Install dependencies
make deps

# Run locally
make run

# Build binary
make build
./bin/server

# Build Docker image
make docker-build

# Deploy to Kubernetes
kubectl apply -f deployments/kubernetes/