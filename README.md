# image-reloader
Subscribe to image changes in k8s and update the controller

## Capabilities
Subscribe to image changes in k8s and update the controller. (Only deployment, statefulset)

Currently available for harbor clundevent webhook or use the script `push_images_and_event.sh` to push the image and simulate harbor sending events to webhook. When pushes an image, it automatically updates resources with the **same image prefix**.

## Quick start
```sh
kubectl apply -f resources/deployments/deploy.yaml
```
push an image and simulate harbor sending events to webhook by `push_images_and_event.sh` (This is especially useful for image repositories without webhooks, such as `registry`.)
```sh
WEBHOOK_URL=http://<nodeIP>:8888/webhook ./push_images_and_event.sh buildah push 127.0.0.1:5000/test/app:v0.0.1
```

## Directory structure
- The `model` directory contains the definition of the data model.
- The `service` directory contains the business logic.
- The `handler` directory contains the HTTP processing function.
