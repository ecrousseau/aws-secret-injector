echo "push tag $1"
docker push erousseau/aws-secrets-injector-init-container:$1
docker push erousseau/aws-secrets-injector-adm-controller:$1
