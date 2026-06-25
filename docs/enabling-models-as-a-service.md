# Enabling Models as a Service (MaaS)

This guide explains how to enable and use Models as a Service (MaaS) with the AI Gateway operator.

## Overview

Models as a Service (MaaS) provides a platform for managing AI/ML model deployments with features including:

- **Multi-tenancy**: Isolated tenants with their own model subscriptions and API keys
- **API Key Management**: Secure authentication and authorization for model access
- **Model Subscriptions**: Control which models are available to which tenants
- **Gateway Integration**: Route model inference requests through a shared Gateway
- **Model Catalog**: Manage both internal and external model references

MaaS is deployed as a sub-component of the AI Gateway operator, alongside batch-gateway.

## Prerequisites

Before enabling MaaS, ensure you have:

- **OpenShift AI** (or Open Data Hub) installed
- **Gateway API CRDs** installed on your cluster
- **ai-gateway-operator** deployed and running
- Cluster admin permissions to create the AIGateway CR

**Optional dependencies** (recommended for full functionality):
- **KServe operator** - for LLMInferenceService integration
- **Kuadrant operator** - for advanced rate limiting and auth policies

> **Note:** Without KServe or Kuadrant installed, maas-controller may experience restart loops due to missing optional CRDs. This is a known upstream issue. The controller remains functional between restarts, but for production use, consider installing these operators or waiting for an upstream fix.

## Enabling MaaS

### Step 1: Check Current AIGateway Configuration

First, check if an AIGateway CR already exists:

```bash
kubectl get aigateway
```

If no AIGateway exists, you'll need to create one. If one exists (typically `default-aigateway`), you'll update it.

### Step 2: Configure AIGateway CR

Create or update the AIGateway custom resource to enable MaaS:

```yaml
apiVersion: components.platform.opendatahub.io/v1alpha1
kind: AIGateway
metadata:
  name: default-aigateway
spec:
  modelsAsService:
    managementState: Managed
```

**Management States:**
- `Managed` - Deploy and manage MaaS components
- `Removed` - Remove MaaS components
- `Unmanaged` - Leave existing MaaS components in place but stop managing them

### Step 3: Apply the Configuration

Apply the AIGateway CR:

```bash
kubectl apply -f aigateway.yaml
```

### Step 4: Verify Deployment

Check that the AIGateway CR is ready:

```bash
kubectl get aigateway default-aigateway -o yaml
```

Expected status:

```yaml
status:
  phase: Ready
  conditions:
  - type: Ready
    status: "True"
  - type: DeploymentsAvailable
    status: "True"
  - type: ProvisioningSucceeded
    status: "True"
```

Check that maas-controller is deployed:

```bash
kubectl get deployment -n ai-gateway-system maas-controller
```

Expected output:

```
NAME              READY   UP-TO-DATE   AVAILABLE   AGE
maas-controller   1/1     1            1           2m
```

**Note:** If you see `CrashLoopBackOff` status with restarts, this is expected when KServe/Kuadrant CRDs are missing. The controller is functional between restarts and will create all necessary resources.

### Step 5: Verify MaaS CRDs

Check that all MaaS custom resource definitions are installed:

```bash
kubectl get crd | grep maas
```

Expected output:

```
aitenants.maas.opendatahub.io
configs.maas.opendatahub.io
externalmodels.maas.opendatahub.io
maasauthpolicies.maas.opendatahub.io
maasmodelrefs.maas.opendatahub.io
maassubscriptions.maas.opendatahub.io
tenants.maas.opendatahub.io
```

### Step 6: Verify Default Tenant

A default tenant is automatically created:

```bash
kubectl get tenant -n models-as-a-service default-tenant
```

Expected output:

```
NAME             READY   REASON
default-tenant   False   GatewayNotReady
```

**Note:** `GatewayNotReady` is expected until the Gateway CR is deployed (see Gateway Configuration below).

## Gateway Configuration

MaaS requires a Gateway CR to route model inference requests. This Gateway is typically deployed by the OpenShift AI platform operator.

### Check Gateway Status

```bash
kubectl get gateway -n openshift-ingress maas-default-gateway
```

If the Gateway doesn't exist, the Tenant will remain in `GatewayNotReady` state. Contact your platform administrator to deploy the Gateway.

### Gateway Requirements

The Gateway CR must:
- Exist in the `openshift-ingress` namespace (or configured namespace)
- Be named `maas-default-gateway` (or match the configured name)
- Be ready and accepting traffic

## Using MaaS

Once MaaS is deployed, you can manage tenants, subscriptions, and models.

### Creating a Tenant

Tenants provide isolation for different teams or projects:

```yaml
apiVersion: maas.opendatahub.io/v1alpha1
kind: Tenant
metadata:
  name: my-team
  namespace: models-as-a-service
spec:
  # Tenant configuration
  displayName: "My Team"
```

Apply with:

```bash
kubectl apply -f tenant.yaml
```

### Managing Model Subscriptions

Subscriptions control which models a tenant can access:

```yaml
apiVersion: maas.opendatahub.io/v1alpha1
kind: MaaSSubscription
metadata:
  name: my-subscription
  namespace: models-as-a-service
spec:
  tenantRef:
    name: my-team
  modelRef:
    name: my-model
  # Additional subscription settings
```

### API Key Management

MaaS provides secure API key generation and management for tenant authentication. API keys are automatically generated and can be rotated through the MaaS API.

(For detailed API key management documentation, see the MaaS controller documentation.)

## Configuration Options

### Image Customization

The MaaS controller deployment uses images specified in a ConfigMap. To customize images:

1. Check current images:

```bash
kubectl get configmap -n ai-gateway-system -l app.kubernetes.io/name=maas-controller -o yaml
```

2. Update the ConfigMap values (typically managed by the platform operator)

### Namespace Configuration

By default, MaaS creates resources in these namespaces:
- `ai-gateway-system` - MaaS controller deployment
- `models-as-a-service` - Tenant subscriptions and model references
- `ai-tenants` - AITenant resources

## Monitoring and Troubleshooting

### Common Issues

#### 1. maas-controller in CrashLoopBackOff

**Symptoms:**
```bash
kubectl get pod -n ai-gateway-system
# maas-controller-xxx   0/1   CrashLoopBackOff   4 (30s ago)
```

**Cause:** Missing optional CRDs (KServe LLMInferenceService, Kuadrant AuthPolicy/TokenRateLimitPolicy)

**Impact:** Controller restarts every ~30 seconds but remains functional between restarts. All CRDs, Tenants, and core functionality work correctly.

**Solutions:**
- **Option 1:** Install KServe operator (provides LLMInferenceService CRD)
- **Option 2:** Install Kuadrant operator (provides AuthPolicy/TokenRateLimitPolicy CRDs)
- **Option 3:** Wait for upstream fix in models-as-a-service repository
- **Option 4:** Accept the restart loop (controller is functional between restarts)

**Check logs:**
```bash
kubectl logs -n ai-gateway-system deployment/maas-controller --tail=50
```

#### 2. Tenant Stuck in GatewayNotReady

**Symptoms:**
```bash
kubectl get tenant -n models-as-a-service default-tenant
# NAME             READY   REASON
# default-tenant   False   GatewayNotReady
```

**Cause:** Missing Gateway CR

**Solution:** Ensure Gateway CR exists:
```bash
kubectl get gateway -n openshift-ingress maas-default-gateway
```

If missing, contact your platform administrator. The Gateway is shared infrastructure deployed by the OpenShift AI operator.

#### 3. AIGateway Not Ready

**Symptoms:**
```bash
kubectl get aigateway default-aigateway
# Status shows Ready: False
```

**Check deployment status:**
```bash
kubectl get deployment -n ai-gateway-system maas-controller
```

**Check ai-gateway-operator logs:**
```bash
kubectl logs -n ai-gateway-system deployment/ai-gateway-operator --tail=50
```

**Common causes:**
- RBAC permission issues (check operator logs for "forbidden" errors)
- Resource constraints (check pod events)
- Image pull failures (check pod status)

#### 4. Missing CRDs

**Verify all CRDs exist:**
```bash
kubectl get crd | grep maas | wc -l
# Should return: 7
```

**If CRDs are missing:**
- Check maas-controller logs
- Verify ai-gateway-operator has permissions to create CRDs
- Check RBAC ClusterRole includes CRD permissions

### Viewing MaaS Resources

**List all MaaS deployments:**
```bash
kubectl get deployments -n ai-gateway-system -l app.kubernetes.io/part-of=models-as-a-service
```

**List all Tenants:**
```bash
kubectl get tenant -A
```

**List all Subscriptions:**
```bash
kubectl get maassubscription -A
```

**List all Model References:**
```bash
kubectl get maasmodelref -A
```

### Debug Information

**Get full AIGateway status:**
```bash
kubectl get aigateway default-aigateway -o yaml
```

**Get maas-controller pod details:**
```bash
kubectl get pod -n ai-gateway-system -l app.kubernetes.io/name=maas-controller -o yaml
```

**Get ConfigMap:**
```bash
kubectl get configmap -n ai-gateway-system -l app.kubernetes.io/name=maas-controller -o yaml
```

## Disabling MaaS

To disable MaaS and remove all components:

```yaml
apiVersion: components.platform.opendatahub.io/v1alpha1
kind: AIGateway
metadata:
  name: default-aigateway
spec:
  modelsAsService:
    managementState: Removed
```

Apply the configuration:

```bash
kubectl apply -f aigateway.yaml
```

This will:
- Remove the maas-controller deployment
- Remove the MaaS webhook service
- Keep the CRDs (for data preservation)
- Keep existing Tenant/Subscription resources

**Note:** To completely remove all MaaS resources including CRDs and custom resources, you'll need to manually delete them:

```bash
# Delete all Tenants, Subscriptions, etc.
kubectl delete tenant --all -A
kubectl delete maassubscription --all -A
kubectl delete maasmodelref --all -A

# Delete CRDs (will delete all instances)
kubectl delete crd aitenants.maas.opendatahub.io
kubectl delete crd configs.maas.opendatahub.io
kubectl delete crd externalmodels.maas.opendatahub.io
kubectl delete crd maasauthpolicies.maas.opendatahub.io
kubectl delete crd maasmodelrefs.maas.opendatahub.io
kubectl delete crd maassubscriptions.maas.opendatahub.io
kubectl delete crd tenants.maas.opendatahub.io
```

## Architecture

For implementation details and architecture information, see:
- [Architecture Documentation](architecture.md)
- [OpenDataHub Integration](integration-opendatahub-operator.md)

## Additional Resources

- **MaaS Controller Documentation**: [models-as-a-service repository](https://github.com/opendatahub-io/models-as-a-service)
- **AI Gateway Operator**: [ai-gateway-operator repository](https://github.com/opendatahub-io/ai-gateway-operator)
- **Gateway API**: [Gateway API documentation](https://gateway-api.sigs.k8s.io/)

## Getting Help

If you encounter issues not covered in this guide:

1. Check the [troubleshooting section](#monitoring-and-troubleshooting) above
2. Review logs from maas-controller and ai-gateway-operator
3. Check the [models-as-a-service issue tracker](https://github.com/opendatahub-io/models-as-a-service/issues)
4. File an issue in the appropriate repository with debug information
