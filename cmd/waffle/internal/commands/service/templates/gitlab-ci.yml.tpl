include:
  - project: 'gotrg/white-label/infrastructure/auto-devops'
    file: 'go/gitlab-ci.yml'

variables:
  HELM_RELEASE_NAME: {{ .Service }}
  K8S_SECRET_ENVIRONMENT: {{ .Service }}

production_manual:
  variables:
    HELM_UPGRADE_VALUES_FILE: .gitlab/auto-deploy-values.yaml
