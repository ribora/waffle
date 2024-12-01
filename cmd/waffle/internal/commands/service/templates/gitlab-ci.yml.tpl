include:
  - project: 'gotrg/white-label/infrastructure/auto-devops'
    file: 'go/gitlab-ci.yml'
  - project: 'gotrg/white-label/infrastructure/auto-devops'
  file: 'project/gitlab-ci.yml'


variables:
  HELM_RELEASE_NAME: {{ .Service }}
  K8S_SECRET_ENVIRONMENT: {{ .Service }}
