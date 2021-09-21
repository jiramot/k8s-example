BINARY=build

build:
	GOOS=linux go build -o bin/app main.go

package:
	docker build -t ghcr.io/jiramot/k8s-example/simple-watch .

run:
	kubectl run --rm -i demo --image=ghcr.io/jiramot/k8s-example/simple-watch --image-pull-policy=Never

