#!/bin/bash
# Copyright 2024 Upbound Inc
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

cleanup_flag=''

print_usage() {
  printf "Usage: -c cleans up all resources on the target MCP, no flags generates random resources."
}

# Check for cleanup
while getopts 'c' flag; do
  case "${flag}" in
    c) cleanup_flag='true' ;;
    *) print_usage
       exit 1 ;;
  esac
done

if [[ $cleanup_flag ]]; then
    echo "Cleaning up all resources on control plane."
    kubectl delete --all Clusters.platform,Databases.platform,Networks.platform,ServiceAccounts.platform,Subnetworks.platform,NodePools.platform,CompositeClusters.platform,AccountScaffolds.platform
    exit 1
fi

# Check for prerequisites to be instsalled
if ! [ -x "$(command -v kcl)" ]; then
      echo "The test harness requires kcl. kcl couldn't be found; please install it first: https://www.kcl-lang.io/docs/user_docs/getting-started/install"
 >&2
  exit 1
fi

# Confirm user wants to create resources against current kubecontext before continuing
echo "This script uses your current kubecontext to create several claims against the APIs defined in this configuration."
read -p "Are you sure you want to continue (y/n)? " CONT
if [ "$CONT" = "y" ]; then
  echo "This scipt will apply a random number of claims for each XR type defined in this configuration.";
else
  exit 1;
fi

# Generate a random number of clusters
random_number=$((1 + RANDOM % 10))
counter=1
while [ $counter -le $random_number ]
do
    clustername=cluster"$counter"
    kcl XCluster/claim.k -D resourcename=$clustername | kubectl apply -f -
    ((counter++))
done

# Generate a random number of databases
random_number=$((1 + RANDOM % 10))
counter=1
while [ $counter -le $random_number ]
do
    databasename=database"$counter"
    kcl XDatabase/claim.k -D resourcename=$databasename | kubectl apply -f -
    ((counter++))
done

# Generate a random number of networks
random_number=$((1 + RANDOM % 10))
counter=1
while [ $counter -le $random_number ]
do
    networkname=network"$counter"
    kcl XNetwork/claim.k -D resourcename=$networkname | kubectl apply -f -
    ((counter++))
done

# Generate a random number of nodepools
random_number=$((1 + RANDOM % 10))
counter=1
while [ $counter -le $random_number ]
do
    nodepoolname=nodepool"$counter"
    kcl XNodePool/claim.k -D resourcename=$nodepoolname | kubectl apply -f -
    ((counter++))
done

# Generate a random number of service accounts
random_number=$((1 + RANDOM % 10))
counter=1
while [ $counter -le $random_number ]
do
    serviceaccountname=serviceaccount"$counter"
    kcl XServiceAccount/claim.k -D resourcename=$serviceaccountname | kubectl apply -f -
    ((counter++))
done

# Generate a random number of subnetworks
random_number=$((1 + RANDOM % 10))
counter=1
while [ $counter -le $random_number ]
do
    subnetworkname=subnetwork"$counter"
    kcl XSubnetwork/claim.k -D resourcename=$subnetworkname | kubectl apply -f -
    ((counter++))
done

# Generate a random number of composite clusters
random_number=$((1 + RANDOM % 10))
counter=1
while [ $counter -le $random_number ]
do
    compositeclustername=cluster"$counter"
    kcl XCompositeCluster/claim.k -D resourcename=$compositeclustername | kubectl apply -f -
    ((counter++))
done

# Generate a random number of account scaffolds
random_number=$((1 + RANDOM % 10))
counter=1
while [ $counter -le $random_number ]
do
    accountscaffoldname=account"$counter"
    kcl XAccountScaffold/claim.k -D resourcename=$accountscaffoldname | kubectl apply -f -
    ((counter++))
done