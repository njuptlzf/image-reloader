FROM docker.io/library/alpine:3.19 as prod

ENV \
    APP_USER=app \
    APP_UID=1001

RUN adduser -s /bin/sh -D -u $APP_UID $APP_USER && chown -R $APP_USER:$APP_USER /home/$APP_USER

COPY _output/bin/image-reloader /usr/local/bin/image-reloader

USER app:app

ENTRYPOINT [ "/usr/local/bin/image-reloader" ]