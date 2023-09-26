# Specify the registry you wish to store the image in
registry = 'sabin1001'

# List the k8s context you wish to run this in
allow_k8s_contexts('capi-quicktest-admin@capi-quicktest')

# Specify docker registry you wish to store image in
docker_build(registry + '/cloud-provider-equinix-metal',
            context='.',
            dockerfile='./Dockerfile',
            ignore=['cloud-sa.json','dev/'],
)

# read in the yaml file and replace the image name with the one we built
deployment = read_yaml_stream('deploy/template/deployment.yaml')
deployment[0]['spec']['template']['spec']['containers'][0]['image'] = registry + '/cloud-provider-equinix-metal'
deployment[0]['spec']['template']['spec']['containers'][0]['env']=[]
deployment[0]['spec']['template']['spec']['containers'][0]['env'].append({"name": "METAL_METRO_NAME","value":"da"})
deployment[0]['spec']['template']['spec']['containers'][0]['env'].append({"name": "METAL_LOAD_BALANCER","value":"emlb:///da"})
k8s_yaml(encode_yaml_stream(deployment))
k8s_resource(workload='cloud-provider-equinix-metal',objects=['cloud-controller-manager:ServiceAccount:kube-system','system\\:cloud-controller-manager:ClusterRole:default','system\\:cloud-controller-manager:ClusterRoleBinding:default'])
k8s_resource(new_name='metal-cloud-config',objects=['metal-cloud-config:Secret:kube-system'])

# Load the secret extension
load('ext://secret', 'secret_create_generic')

# Create the cloud-provider-equinix-metal secret based on the contents of the 
# file named cloud-sa.json put the apiKey and projectID in it
# The file should look like this:
# {
#      "apiKey":"YOUR_API_KEY",
#      "projectID":"YOUR_PROJECT_ID"
# }
secret_create_generic(
    'metal-cloud-config',
    'kube-system',
    from_file='cloud-sa.json=./cloud-sa.json'
)