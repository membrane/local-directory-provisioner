pipeline {
    agent any

    stages {
        stage('Docker build') {
            steps {
                sh 'docker build -t p8/local-directory-provisioner .'
            }
        }
        stage('Docker tag') {
            steps {
                sh 'docker tag p8/local-directory-provisioner hub.predic8.de/p8/local-directory-provisioner:$BUILD_NUMBER'
                sh 'docker tag p8/local-directory-provisioner hub.predic8.de/p8/local-directory-provisioner:latest'
            }
        }
        stage('Docker push') {
            steps {
                sh 'docker push hub.predic8.de/p8/local-directory-provisioner:$BUILD_NUMBER'
                sh 'docker push hub.predic8.de/p8/local-directory-provisioner:latest'
            }
        }
        stage('Apply Service Account') {
            steps {
                sh 'kubectl $KUBECTL_OPTS apply -f ./ldp/deploy/rbac/serviceaccount.yaml'
            }
        }
        stage('Apply Storage Claim') {
            steps {
                sh 'kubectl $KUBECTL_OPTS apply -f ./ldp/deploy/rbac/storageclass.yaml'
            }
        }
        stage('Apply Cluster Role') {
            steps {
                sh 'kubectl $KUBECTL_OPTS apply -f ./ldp/deploy/rbac/clusterrole.yaml'
            }
        }
        stage('Apply Cluster Role Binding') {
            steps {
                sh 'kubectl $KUBECTL_OPTS apply -f ./ldp/deploy/rbac/clusterrolebinding.yaml'
            }
        }
        stage('Apply Daemon Set') {
            steps {
                sh 'kubectl $KUBECTL_OPTS apply -f ./ldp/deploy/rbac/daemonset.yaml'
            }
        }
    }
}
