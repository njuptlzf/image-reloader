# image-reloader
Quickly update kubernetes image based on pull/push mode

## Capabilities
Currently it is used with harbor/webhook cloundevent, watch deployent, statefulset. When harbor pushes an image, it automatically updates resources with the same image prefix

## Directory structure
- The `model` directory contains the definition of the data model.
- The `service` directory contains the business logic.
- The `handler` directory contains the HTTP processing function.
