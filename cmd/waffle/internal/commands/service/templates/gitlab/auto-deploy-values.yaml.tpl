fullnameOverride: {{ .Service }}

readinessProbe:
  exec:
    command:
      - health

livenessProbe:
  exec:
    command:
      - health

ingress:
  enabled: false

service:
  internalPort: 8009

prometheus:
  metrics: true

resources:
  requests:
    cpu: 100m
    memory: 512Mi
  limits:
    cpu: 500m
    memory: 1Gi

jobs:
  upgrade:
    annotations:
      "helm.sh/hook": post-install,post-upgrade
      "helm.sh/hook-weight": "-5"
      "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
    command:
      - app
      - migrate
      - up
    resources:
      requests:
        cpu: 50m
        memory: 100Mi
