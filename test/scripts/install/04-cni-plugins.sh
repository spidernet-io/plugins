#!/bin/bash

set -o errexit -o nounset -o xtrace

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)
PROJECT_ROOT_PATH=$( cd ${CURRENT_DIR_PATH}/../../.. && pwd )

ARCH=`uname -m`
if [ ${ARCH} == "x86_64" ]; then ARCH="amd64" ; fi

DOWNLOAD_DIR=${PROJECT_ROOT_PATH}/.tmp/plugins
if [ ! -d "${DOWNLOAD_DIR}" ]; then mkdir -p ${DOWNLOAD_DIR} ; fi

# prepare cni-plugins
PACKAGE_NAME="cni-plugins-linux-${ARCH}-${CNI_PLUGINS_VERSION}.tgz"

DOWNLOAD_URL="https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/${PACKAGE_NAME}"
if [ ${RUN_ON_LOCAL} == "true" ]; then DOWNLOAD_URL=https://ghproxy.com/${DOWNLOAD_URL} ; fi
if [ ! -f  "${PROJECT_ROOT_PATH}/.tmp/plugins/${PACKAGE_NAME}" ]; then
  echo "begin to download cni-plugins ${PACKAGE_NAME} "
  wget -P ${DOWNLOAD_DIR} ${DOWNLOAD_URL}
else
  echo "${DOWNLOAD_DIR}/${PACKAGE_NAME} exist, Skip download"
fi

CNI_PACKAGE_PATH=${PROJECT_ROOT_PATH}/.tmp/plugins/${PACKAGE_NAME}

echo ${CNI_PACKAGE_PATH}

ls /home/spider-plugins-*.tar
[ "$?" != "0"] && echo "spider plugins no found" && exit 2
SPIDER_PLUGINS_FILES_PATH=`ls /home/spider-plugins-*.tar`

kind_nodes=`docker ps  | egrep "kindest/node.* ${IP_FAMILY}-(control-plane|worker)"  | awk '{print $1}'`
for node in ${kind_nodes} ; do
  echo "install cni-plugins to kind-node: ${node} "
  docker cp ${CNI_PACKAGE_PATH} $node:/root/
  docker exec $node tar xvfzp /root/${PACKAGE_NAME} -C /opt/cni/bin
  docker cp ${SPIDER_PLUGINS_FILES_PATH} $node:/root/
  docker exec $node tar xvfzp /root/${SPIDER_PLUGINS_FILES_PATH} -C /opt/cni/bin
done

echo -e "\033[35m Succeed to install cni-plugins to kind-node \033[0m"
