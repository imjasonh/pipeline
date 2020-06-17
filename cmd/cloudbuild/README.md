# Google Cloud Build for Tekton

## Install

Install and configure `ko`.

```
ko apply -f controller.yaml
```

This will build and install the controller on your cluster.

## Service Account Setup

Builds need to be created by a Service Account with appropriate permissions.

[Create a GCP Service
Account](https://cloud.google.com/kubernetes-engine/docs/tutorials/authenticating-to-cloud-platform#step_3_create_service_account_credentials)
with the **Cloud Build Editor** role, create and download a key file for that
SA, and create a K8s Secret with that Service Account Key:

```
kubectl create secret generic sa-key --from-file=key.json=PATH-TO-KEY-FILE.json -n cloudbuild-task
```

## Run a Build

Create a `Run` that refers to a Build:

```
$ kubectl create -f gcb-run.yaml 
run.tekton.dev/gcb-run-j2w5p created
$ kubectl get runs -w
NAME            SUCCEEDED   REASON         STARTTIME   COMPLETIONTIME
gcb-run-j2w5p   Unknown     BuildWorking   15s         
gcb-run-j2w5p   True        BuildSucceeded   31s         1s
```
