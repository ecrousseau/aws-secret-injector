echo "build with tag $1"
docker build admission-controller -t erousseau/aws-secrets-injector-adm-controller:$1
docker build init-container -t erousseau/aws-secrets-injector-init-container:$1
