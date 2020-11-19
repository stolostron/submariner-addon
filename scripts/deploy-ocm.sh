#!/bin/bash

work_dir="$(pwd)/_output"
rm -rf ${work_dir}
mkdir -p ${work_dir}
registration_operator_dir="${work_dir}/registration-operator"
cluster="cluster1"

# downlaod registration-operator
git clone https://github.com/open-cluster-management/registration-operator.git ${registration_operator_dir}

if [ "$1"x = "cleanup"x ]; then
    cd ${registration_operator_dir}
    make clean-deploy
    exit 0
fi

cd ${registration_operator_dir}
make deploy

for i in {0..60}; do
  kubectl get managedclusters ${cluster} >/dev/null 2>&1
  if [ 0 -eq $? ]; then
    break
  fi

  if [ $i -eq 60 ]; then
    echo "Unable to find managed cluster ${cluster} within 5 min"
      return 2
  fi
    sleep 5

done

# accept the managed cluster
kubectl patch managedclusters ${cluster} --type merge --patch '{"spec":{"hubAcceptsClient":true}}'

# approve the managed cluster csr
csr_name=$(kubectl get csr |grep "${cluster}" | grep "Pending" |awk '{print $1}')
kubectl certificate approve "${csr_name}"

echo "Wait managed clusters are joined ..."
for i in {0..60}; do
  jcounts=$(kubectl get managedclusters ${cluster} | awk '{print $4}' | grep "True" | wc -l)
  if [ ${jcounts} -eq 1 ]; then
      kubectl get managedclusters ${cluster}
      break
  fi

   if [ $i -eq 60 ]; then
    echo "managed cluster ${cluster} is not joined within 5 min"
    return 2
  fi
  sleep 5
done
