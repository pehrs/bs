please do not make mistakes and do not lie.
please verify the code you write works.

please create an interactive command line user interface (tui) in golang that can list and show details of catalog entities from a backstage backend server

please use the bubbletea library (https://github.com/charmbracelet/bubbletea) for the golang application

Here's an example on how to get entities using curl:
```shell
ACCESS_TOKEN=should-be-secret
KIND=user

curl -s -X GET \
     -H "Authorization: Bearer ${ACCESS_TOKEN}" \
     "http://localhost:7007/api/catalog/entities/by-query?filter=kind=${KIND},metadata.namespace=default&filter=kind=${KIND},spec.type"
```
     
The definition of Backstage catalog entities can be found on this page: https://backstage.io/docs/features/software-catalog/descriptor-format

