package main

import (
    "fmt"
    "os"
    "strings"
    "strconv"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/arn"
    "github.com/aws/aws-sdk-go/aws/awserr"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/secretsmanager"
    "k8s.io/klog"
)

type Secret struct {
    Id string
    Region string
    ExplodeJson bool
}

func main() {
    envSecretArns := os.Getenv("SECRET_ARNS")
    envSecretNames :=  os.Getenv("SECRET_NAMES")
    envSecretRegion := os.Getenv("SECRET_REGION")
    envExplodeJsonKeys := os.Getenv("EXPLODE_JSON_KEYS")
    explodeJsonKeys := false
    parsedEnvExplodeJsonKeys, err := strconv.ParseBool(envExplodeJsonKeys)
    if !err {
        explodeJsonKeys = parsedEnvExplodeJsonKeys
    }
    var secrets []Secret
    if envSecretArns != "" { 
        klog.Info("SECRET_ARNS env var is ", envSecretArns)
        for _, secretArn := range strings.Split(envSecretArns, ",") {
            if !arn.IsARN(secretArn) {
                klog.Error("Invalid ARN: ", secretArn)
                continue
            }
            parsedArn, _ := arn.Parse(secretArn)
            secrets = append(secrets, Secret{
                Id: secretArn,
                Region: parsedArn.Region,
                ExplodeJson: envExplodeJsonKeys,
            })
        }
    } else if envSecretNames != "" {
        klog.Info("SECRET_NAMES env var is ", envSecretNames, " and SECRET_REGION is ", envSecretRegion)
        for _, name := range strings.Split(envSecretNames, ",") {
            secrets = append(secrets, Secret{
                Id: name,
                Region: envSecretRegion,
                ExplodeJson: envExplodeJsonKeys,
            })
        }
    } else {
        klog.Error("Unable to read environment variables SECRET_ARNS or SECRET_NAMES")
    }
    for _, secret := range secrets {
        klog.Info("Processing: ", secret.Id)
        getSecretValue(secret)
        klog.Info("Done processing: ", secret.Id)
    }
}

func getSecretValue(secret Secret) {
    sess := session.Must(session.NewSession())
    svc := secretsmanager.New(sess, &aws.Config{
        Region: aws.String(secret.Region),
    })
    input := &secretsmanager.GetSecretValueInput{
        SecretId: aws.String(secret.Id),
    }
    result, err := svc.GetSecretValue(input)
    if err != nil {
        if aerr, ok := err.(awserr.Error); ok {
            switch aerr.Code() {
            case secretsmanager.ErrCodeResourceNotFoundException:
                klog.Error(secretsmanager.ErrCodeResourceNotFoundException, aerr.Error())
            case secretsmanager.ErrCodeInvalidParameterException:
                klog.Error(secretsmanager.ErrCodeInvalidParameterException, aerr.Error())
            case secretsmanager.ErrCodeInvalidRequestException:
                klog.Error(secretsmanager.ErrCodeInvalidRequestException, aerr.Error())
            case secretsmanager.ErrCodeDecryptionFailure:
                klog.Error(secretsmanager.ErrCodeDecryptionFailure, aerr.Error())
            case secretsmanager.ErrCodeInternalServiceError:
                klog.Error(secretsmanager.ErrCodeInternalServiceError, aerr.Error())
            default:
                klog.Error(aerr.Error())
            }
        } else {
            klog.Error(err)
        }
        return
    }
    if result.SecretString != nil {
        if secret.ExplodeJson {
            writeJsonOutput(*result.Name, *result.SecretString)
        } else {
            writeStringOutput(*result.Name, *result.SecretString)
        }
    } else {
        writeBinaryOutput(*result.Name, result.SecretBinary)
    }
}

func writeJsonOutput(name string, output string) {
    klog.Info("Exploding json data from SecretString into files")

    var result map[string]interface{}
    errJson := json.Unmarshal([]byte(output), &result)
    //In case the body is not json
    if errJson != nil {
        klog.Info("The SecretString is not json")
        f, err := os.Create(fmt.Sprintf("/injected-secrets/%s", name))
        if err != nil {
            klog.Error(err)
            return
        }
        defer f.Close()
        len, err := f.WriteString(output)
        if err != nil {
            klog.Error(err)
            return
        } else {
            klog.Info(fmt.Sprintf("Wrote %d bytes to /injected-secrets/%s", len, name))
        }
    } else {
        for key, value := range result {
            // Each value is an interface{} type, that is type asserted as a string
            f, err := os.Create(fmt.Sprintf("/injected-secrets/%s/%s", name, key))
            if err != nil {
                klog.Error(err)
                return
            }
            defer f.Close()
            len, err := f.WriteString(value.(string))
            if err != nil {
                klog.Error(err)
                return
            } else {
                klog.Info(fmt.Sprintf("Wrote %d bytes to /injected-secrets/%s/%s", len, name, key))
            }
        }
    }
}

func writeStringOutput(name string, output string) {
    klog.Info("Writing data from SecretString")
    f, err := os.Create(fmt.Sprintf("/injected-secrets/%s", name))
    if err != nil {
        klog.Error(err)
        return
    }
    defer f.Close()
    len, err := f.WriteString(output)
    if err != nil {
        klog.Error(err)
        return
    } else {
        klog.Info(fmt.Sprintf("Wrote %d bytes to /injected-secrets/%s", len, name))
    }
}

func writeBinaryOutput(name string, output []byte) {
    klog.Info("Writing data from SecretBinary")
    f, err := os.Create(fmt.Sprintf("/injected-secrets/%s", name))
    if err != nil {
        klog.Error(err)
        return
    }
    defer f.Close()
    len, err := f.Write(output)
    if err != nil {
        klog.Error(err)
        return
    } else {
        klog.Info(fmt.Sprintf("Wrote %d bytes to /injected-secrets/%s", len, name))
    }
}
