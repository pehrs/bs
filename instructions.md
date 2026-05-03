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

---

Change the main.go implementation so that the main menu when starting the application does not call the backend directly but the user first can choose the type of entities to view.

---

Change the main.go implementation to support pagination of the backstage responses as described on this page: https://backstage.io/docs/features/software-catalog/software-catalog-api/

---

change the implementation and move the current top level menu to its own file called listall.go and make a new top level menu that can naviagte to that. 

---

create a new top level page that searches backstage for entities and presents the result.
Here's an example on how to search backstage with curl:
```shell
ACCESS_TOKEN=should-be-secret
SEARCH_TERM="service"

curl -s -X GET \
     -H "Authorization: Bearer ${ACCESS_TOKEN}" \
     "http://localhost:7007/api/catalog/entities/by-query?fullTextFilterTerm=${SEARCH_TERM}&fullTextFilterFields=metadata.name,metadata.title"
```