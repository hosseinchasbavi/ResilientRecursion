# ResilientRecursion

ResilientRecursion is a distributed computing application designed to compute recursive functions efficiently in a Kubernetes environment. The application leverages Redis for caching, Go for high-performance computation, and Kubernetes for scalability and fault tolerance.

---

## **Table of Contents**
- Overview
- Features
- Architecture
- Installation
- Usage
- API Endpoints
- Configuration
- Deployment on Kubernetes
- Contributing
- License

---

## **Overview**
The goal of this project is to compute the sum of values from a recursive function:

```
xn+1[r] = r * xn[r] * (1 - xn[r]), x0[r] = 0.5
```

The application is designed to handle distributed computation with limited resources, ensuring efficient resource utilization and scalability.

---

## **Features**
- Compute recursive values for a given `r` and `n`.
- Cache intermediate results in Redis for faster computation.
- Preheat cache on startup to reduce cold-start latency.
- Graceful shutdown with cache flushing to Redis.
- Scalable and fault-tolerant deployment on Kubernetes.
- RESTful API for interacting with the application.

---

## **Architecture**
The application consists of the following components:
1. **Compute Engine**:
   - Handles recursive computations.
   - Uses an in-memory L1 cache for fast access.
   - Stores checkpoints in Redis for persistence.

2. **Redis**:
   - Acts as a distributed cache for storing intermediate results and checkpoints.

3. **Kubernetes**:
   - Manages the lifecycle of the application.
   - Ensures scalability and high availability.

4. **REST API**:
   - Provides endpoints for clients to submit computation requests.

---

## **Installation**

### **Prerequisites**
- Go 1.21 or higher
- Docker
- Kubernetes (Minikube, Kind, or a cloud provider)
- Redis

### **Clone the Repository**
```bash
git clone https://github.com/hosseinchasbavi/ResilientRecursion.git
cd ResilientRecursion
```

### **Build the Application**
```bash
go build
```

### **Run Locally**
```bash
.\resilientrecursion.exe
```

---

## **Usage**

### **Run the Application**
1. Start Redis:
   ```bash
   docker run -d -p 6379:6379 --name redis redis:6.2
   ```

2. Run the application:
   ```bash
   go run main.go
   ```

3. Access the API at `http://localhost:2586`.

---

## **API Endpoints**

### **1. POST `/calculate`**
Compute the sum of recursive values for given `r` and `n`.

#### **Request Body**:
```json
[
    { "r": 4, "n": 1 },
    { "r": 4, "n": 2 },
    { "r": 3.5, "n": 3 }
]
```

#### **Response**:
```json
{
    "result": 1.5
}
```

---

## **Configuration**
The application uses environment variables for configuration:

| Variable       | Default Value   | Description                     |
|----------------|-----------------|---------------------------------|
| `PORT`         | `2586`          | HTTP server port               |
| `REDIS_ADDR`   | `localhost:6379`| Redis server address           |
| `POD_ID`       | `pod-0`         | Unique identifier for the pod  |
| `TOTAL_PODS`   | `3`             | Total number of pods in cluster|

---

## **Deployment on Kubernetes**

### **1. Build and Push Docker Image**
```bash
docker build -t hosseinchb/resilientrecursion:latest .
docker push hosseinchb/resilientrecursion:latest
```

### **2. Deploy Redis**

Apply the Redis deployment:
```bash
kubectl apply -f redis.yaml
```

### **3. Deploy the Application**
Apply the 

deployment.yml

 and `service.yml` files:
```bash
kubectl apply -f deployments/kubernetes/deployment.yml
kubectl apply -f deployments/kubernetes/service.yml
```

### **4. Verify the Deployment**
Check the status of the deployment and service:
```bash
kubectl get deployments
kubectl get services
```

---

## **Contributing**
Contributions are welcome! Please fork the repository and submit a pull request.

---

## **License**
This project is licensed under the MIT License. See the `LICENSE` file for details.

---

### **Best of Luck!**
Enjoy using ResilientRecursion! If you encounter any issues, feel free to open an issue on GitHub.