---

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

---

Add suport for filtering and sorting in all the resulting lists.

---

Add support for reverse sorting by using capital letter S.

---

On the detailed page for an entity if the entity has an annotation called 'backstage.io/techdocs-ref' then add function to render the markdown techdocs for the component using the golang glow library (https://github.com/charmbracelet/glow).

Use the backstage.io annotations to figure out where to read the markdown files from. You should be able to use the backstage.io/managed-by-origin-location and/or backstage.io/techdocs-ref annotations to figure out where the mkdocs.yml file should be read from.

Here are some references to backstage:
- https://backstage.io/docs/features/software-catalog/well-known-annotations/
- https://backstage.io/docs/features/techdocs/creating-and-publishing/

---

the current implementation fails to get the markdown for techdocs if the annotation backstage.io/managed-by-origin-location starts with file://. please fix this.

---

Make techdocs page rendering markdown to work like a proper browser that follows markdown links and renders them correctly as described by this page: https://backstage.io/docs/features/techdocs/how-to-guides/

---

The techdocs markdown rendering is really slow compared to running glow directly. please fix the performance issue.  please explain the reason for the performance issue.

---

Add a toplevel page that does generic search. Here's an example of what the search url looks like using curl: 
```shell
ACCESS_TOKEN=should-be-secret
SEARCH_TERM="service"

curl -s -X GET \
     -H "Authorization: Bearer ${ACCESS_TOKEN}" \
     "http://localhost:7007/api/search/query?term=${SEARCH_TERM}"
```

The file bs-search-sample-response.json shows what the response looks like.

---
