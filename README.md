# image-reloader
Subscribe to image changes in k8s and update the controller

## Capabilities
Currently it is used with harbor/webhook cloundevent, watch deployent, statefulset. 

1. Configure the harbor project's webhook
2. When harbor pushes an image, it automatically updates resources with the same image prefix

## Directory structure
- The `model` directory contains the definition of the data model.
- The `service` directory contains the business logic.
- The `handler` directory contains the HTTP processing function.
