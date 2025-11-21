# generate host key
step ssh certificate --host --no-password --insecure --principal localhost --principal 192.168.1.100 rpi3 ssh_host_ecdsa_key --provisioner oidc --force

# generate user key
step ssh certificate viet viet_ecdsa --no-agent --context prod --no-password --insecure --not-after 24h --provisioner oidc --force

# ssh -F ./ssh_config -i ./viet_ecdsa viet@192.168.1.100

# Proxy
ssh -i data/viet_ecdsa -o 'proxycommand socat - PROXY:10.100.17.215:%h:%p,proxyport=5002' viet@macair