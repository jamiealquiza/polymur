def app = 'polymur'
def namespace = 'techops'
def registry = "registry.revinate.net/${namespace}"
def gopath = "/go/src/github.com/chrissnell/${app}"
def name = "${registry}/${app}"

stage 'Golang build'
node {
  checkout scm

  usingDocker {
    sh "docker run --rm -v `pwd`:${gopath} -w ${gopath} golang:latest bash -c make"
  }

  stash name: 'binary-polymur', includes: "polymur"
  stash name: 'binary-polymur-gateway', includes: "polymur-gateway"
  stash name: 'binary-polymur-proxy', includes: "polymur-proxy"
}

stage 'Docker build and push'
node {
  checkout scm
  unstash "binary-polymur"
  unstash "binary-polymur-gateway"
  unstash "binary-polymur-proxy"

  version = currentVersion()
  hoister.registry = registry
  hoister.imageName = "polymur"
  hoister.buildAndPush version

  stagehandPublish("polymur", version)
}

stage 'Kubernetes deploy to test'
node {
  deployToTest("${app}", "${namespace}")
}
