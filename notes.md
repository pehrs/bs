
# Backstage notes

## Access

Add the following to your app-config.local.yaml

```yaml
backend:
  auth:
    externalAccess:
      - type: static
        options:
          token: ${REPLACE_WITH_A_PRESHARED_SECRET}
          subject: bs
        # accessRestrictions:
        #   - plugin: catalog
        #   - plugin: events
```

## Techdocs

Setup techdocs for local execution (app-config.local.yaml)
```yaml
techdocs:
  builder: 'local'
  generator:
    runIn: 'local'
  publisher:
    type: 'local'
```

Add mkdocs for local execution before starting backstage with yarn

```shell
pyenv local 3.12.5
python -m venv venv
source venv/bin/activate
python -m pip install mkdocs-techdocs-core

```

