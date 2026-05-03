# bs - the friendly backstage cli

`bs` is a simple TUI for [backstage](https://github.com/backstage/backstage) to do quick queries and search in backstage without leaving the terminal. It's not meant as a replacement for the Backstage frontend.

This project was slowly curated together with Claude code over a few sessions with small [incremental prompts](instructions.md) and some corrections by me.


## Setup Backstage for bs access

Add the following to your app-config.local.yaml

```yaml
backend:
  auth:
    externalAccess:b
      - type: static
        options:
          token: ${REPLACE_WITH_A_PRESHARED_SECRET}
          subject: bs
        # FIXME: We SHOULD set restricted access for the token...
        # accessRestrictions:
        #   - plugin: catalog
        #   - plugin: events
```

### Techdocs

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
cd /path/to/backstage

pyenv local 3.12.5
python -m venv venv
source venv/bin/activate
python -m pip install mkdocs-techdocs-core

yarn start

```

