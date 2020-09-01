{{/*
Generate certificate for webhook
*/}}
{{- define "secret-inject.gen-certs" -}}
{{- $hostname := .Release.Namespace | printf "secret-inject.%s.svc" -}}
{{- $ca := genCA "secret-inject-ca" 3650 -}}
{{- $cert := genSignedCert $hostname nil nil 3650 $ca -}}
caCert: {{ $ca.Cert | b64enc }}
clientCert: {{ $cert.Cert | b64enc }}
clientKey: {{ $cert.Key | b64enc }}
{{- end -}}

