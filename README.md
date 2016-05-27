# What is cf-plugin-deploy?

**cf-plugin-deploy** allows you to specify initial users and their credentials, their role/space associations, as well as limiting what brokers are accessible from which spaces at the time of the CF deployment.

## How to install 

Clone the repo, build the binary, and install the plugin:

```
git clone https://github.com/cloudfoundry-community/cf-plugin-deploy
cd cf-plugin-deploy
make 
make cf
```

## How to use

Please see the manifests in `examples` for available syntax. Once you have built the manifest, deploy the changes to your Cloud Foundry with `cf deploy`.
