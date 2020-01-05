`docker run \
-e FILE_MONGO_CONN_STRING="mongodb://mongo:27017/devDB" \
-e FILE_SERVICE_URL="file-service:8080" \
-e PERMISSION_MONGO_CONN_STRING="mongodb://mongo:27017/permission" \
-e SEARCH_SERVICE_URL="search-service:8080" \
--network=web-ui_default \
migration:latest`


`kubectl apply -f job.yaml --namepsace <namespace>`