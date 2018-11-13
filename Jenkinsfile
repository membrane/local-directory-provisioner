pipeline {
    agent any

    stages {
        stage('Docker build') {
            steps {
                sh 'docker build -t p8/local-directory-provisioner/ldp .'
            }
        }

        stage('Docker tag') {
            steps {
                sh 'docker tag p8/local-directory-provisioner/ldp hub.predic8.de/p8/local-directory-provisioner/ldp:$BUILD_NUMBER'
                sh 'docker tag p8/local-directory-provisioner/ldp hub.predic8.de/p8/local-directory-provisioner/ldp:latest'
            }
        }

        stage('Docker push') {
            steps {
                sh 'docker push hub.predic8.de/p8/local-directory-provisioner/ldp:$BUILD_NUMBER'
                sh 'docker push hub.predic8.de/p8/local-directory-provisioner/ldp:latest'
            }
        }
    }
}
