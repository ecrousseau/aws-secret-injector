{{/*
Generate certificate for webhook
*/}}
{{- define "aws-secret-injector.gen-certs" -}}
{{- $hostname := .Release.Namespace | printf "aws-secret-injector.%s.svc" -}}
{{- $ca := genCA "aws-secret-injector-ca" 3650 -}}
{{- $cert := genSignedCert $hostname nil nil 3650 $ca -}}
caCert: {{ $ca.Cert | b64enc }}
clientCert: {{ $cert.Cert | b64enc }}
clientKey: {{ $cert.Key | b64enc }}
{{- end -}}

