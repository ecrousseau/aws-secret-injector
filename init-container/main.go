package main

import (
    "fmt"
    "os"
    "strings"
    "strconv"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/arn"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/aws/endpoints"
    "github.com/aws/aws-sdk-go/service/secretsmanager"
    "k8s.io/klog"
    "encoding/json"
)

// Secret is used to represent the secrets that are to be retrieved and written to file.
type Secret struct {
    Id string
    Region string
    ExplodeJson bool
}


// main is the entry point for the init container.
func main() {
    envSecretArns := os.Getenv("SECRET_ARNS")
    envSecretNames :=  os.Getenv("SECRET_NAMES")
    envSecretRegion := os.Getenv("SECRET_REGION")
    envExplodeJsonKeys := false
    if os.Getenv("EXPLODE_JSON_KEYS") != "" {
        parsedEnvExplodeJsonKeys, err := strconv.ParseBool(os.Getenv("EXPLODE_JSON_KEYS"))
        if err != nil {
            klog.Error("EXPLODE_JSON_KEYS env var could not be parsed")
            os.Exit(1)
        } else {
            envExplodeJsonKeys = parsedEnvExplodeJsonKeys
        }
    }
    var secrets []Secret
    if envSecretArns != "" { 
        klog.Info("SECRET_ARNS env var is ", envSecretArns)
        for _, secretArn := range strings.Split(envSecretArns, ",") {
            if !arn.IsARN(secretArn) {
                klog.Error("Invalid ARN: ", secretArn)
                os.Exit(2)
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
        os.Exit(3)
    }
    stsRegionalEndpoint, _ := endpoints.GetSTSRegionalEndpoint("legacy")
    if os.Getenv("AWS_STS_REGIONAL_ENDPOINTS") != "" {
        parsedSTSRegionalEndpoint, err := endpoints.GetSTSRegionalEndpoint(os.Getenv("AWS_STS_REGIONAL_ENDPOINTS"))
        if err != nil {
            klog.Error("AWS_STS_REGIONAL_ENDPOINTS env var could not be parsed")
            os.Exit(4)
        }
        stsRegionalEndpoint = parsedSTSRegionalEndpoint
    }
    awsSession, err := session.NewSessionWithOptions(session.Options{
        Config: aws.Config{
            CredentialsChainVerboseErrors: aws.Bool(true),
            STSRegionalEndpoint: stsRegionalEndpoint,
        },
    })
    if err != nil {
        klog.Info("Error while creating AWS session: ", err)
        os.Exit(5)
    }
    for _, secret := range secrets {
        klog.Info("Processing: ", secret.Id)
        err := getSecretValue(awsSession, secret)
        if err != nil {
            klog.Info("Error while processing: ", secret.Id)
            os.Exit(6)
        }
        klog.Info("Done processing: ", secret.Id)
    }
}

// getSecretValue retrieves secrets from AWS Secrets Manager and writes the values to files.
func getSecretValue(awsSession *session.Session, secret Secret) error {
    svc := secretsmanager.New(awsSession, &aws.Config{Region: aws.String(secret.Region)})
    input := &secretsmanager.GetSecretValueInput{
        SecretId: aws.String(secret.Id),
    }
    result, err := svc.GetSecretValue(input)
    if err != nil {
        klog.Error("Error while getting secret value: ", err)
        return err
    }
    if result.SecretString != nil {
        if secret.ExplodeJson {
            return writeJsonOutput(*result.Name, *result.SecretString)
        } else {
            return writeStringOutput(*result.Name, *result.SecretString)
        }
    } else {
        return writeBinaryOutput(*result.Name, result.SecretBinary)
    }
}

// writeJsonOutput writes a JSON string representing a map of key-value pairs to a set of files.
// The files are named according to the keys.
// Complex values are re-encoded as JSON.
func writeJsonOutput(name string, output string) error {
    klog.Infof("Exploding json data from %s into files", name)
    var result map[string]interface{}
    err := json.Unmarshal([]byte(output), &result)
    if err != nil {
        klog.Warningf("Value for %s could not be parsed as JSON and will be written directly to file", name)
        writeStringOutput(name, output)
    } else {
        for key, value := range result {
            valueString, ok := value.(string)
            if ok {
                writeStringOutput(fmt.Sprintf("%s/%s", name, key), valueString)
            } else {
                klog.Warningf("Unable to convert value for %s[%s] to string, encoding it as JSON", name, key)
                valueBytes, err := json.Marshal(value)
                if err != nil {
                    klog.Errorf("Error encoding value of %s[%s] to JSON: %s", name, key, err)
                    return err
                }
                writeBinaryOutput(fmt.Sprintf("%s/%s", name, key), valueBytes)
            }
        }
    }
    return nil
}

// writeStringOutput writes a string to file.
func writeStringOutput(name string, output string) error {
    klog.Infof("Writing string data to %s", name)
    f, err := os.Create(fmt.Sprintf("/injected-secrets/%s", name))
    if err != nil {
        klog.Errorf("Error creating file /injected-secrets/%s: %s", name, err)
        return err
    }
    defer f.Close()
    len, err := f.WriteString(output)
    if err != nil {
        klog.Errorf("Error writing to file /injected-secrets/%s: %s", name, err)
        return err
    } else {
        klog.Infof("Wrote %d bytes to /injected-secrets/%s", len, name)
    }
    return nil
}

// writeBinaryOutput writes a slice of bytes to file.
func writeBinaryOutput(name string, output []byte) error {
    klog.Infof("Writing binary data to /injected-secrets/%s", name)
    f, err := os.Create(fmt.Sprintf("/injected-secrets/%s", name))
    if err != nil {
        klog.Errorf("Error creating file /injected-secrets/%s: %s", name, err)
        return err
    }
    defer f.Close()
    len, err := f.Write(output)
    if err != nil {
        klog.Errorf("Error writing to file /injected-secrets/%s: %s", name, err)
        return err
    } else {
        klog.Infof("Wrote %d bytes to /injected-secrets/%s", len, name)
    }
    return nil
}
