## Minikube
```
eval $(minikube docker-env)
make package
kubectl create clusterrolebinding default-view --clusterrole=view --serviceaccount=default:default
make run
```