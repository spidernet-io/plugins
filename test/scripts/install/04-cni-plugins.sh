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

ls ${PROJECT_ROOT_PATH}/.tmp/spider-plugins-linux-amd64-*.tar
[ "$?" != "0" ] && echo "spider plugins no found" && exit 2
SPIDER_PLUGINS_FILE_PATH=`ls ${PROJECT_ROOT_PATH}/.tmp/spider-plugins-linux-amd64-*.tar`
SPIDER_PLUGINS_FILE_NAME=${SPIDER_PLUGINS_FILE_PATH##*/}
mv ${PROJECT_ROOT_PATH}/.tmp/${SPIDER_PLUGINS_FILE_NAME} ${PROJECT_ROOT_PATH}/.tmp/plugins/

ls ${PROJECT_ROOT_PATH}/.tmp/plugins/

kind_nodes=`docker ps  | egrep "kindest/node.* ${IP_FAMILY}-(control-plane|worker)"  | awk '{print $1}'`
for node in ${kind_nodes} ; do
  docker cp ${PROJECT_ROOT_PATH}/.tmp/plugins/${PACKAGE_NAME} $node:/root/
  docker exec $node tar xvfzp /root/${PACKAGE_NAME} -C /opt/cni/bin
  docker cp ${PROJECT_ROOT_PATH}/.tmp/plugins/${SPIDER_PLUGINS_FILE_NAME} $node:/root/
  docker exec $node tar -xvf /root/${SPIDER_PLUGINS_FILE_NAME} -C /opt/cni/bin
done

echo -e "\033[35m Succeed to install cni-plugins to kind-node \033[0m"
