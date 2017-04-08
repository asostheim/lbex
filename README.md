# NGINX Based External Kubernetes Service Load Balancer
## Overview
The Load Balancer External (LBEX) project was built to attempt to resolve a specific problem (described below).  As work progressed on LBEX, it was easy to see simple extensions that made it potentially useful for solving a larger set of problems.  The solution is built around the standard Kubernetes 1.5+ release and the community version of NGINX.  The use of NGINX in this project is entirely due to the UDP load balancing capabilities.  Providing support for both TCP and UDP protocols is a minimum requirement for being able to support general Kubernetes Services.  

In general, LBEX provides the ability to:
- Service network traffic on any Linux distribution that supports the installation of NGINX including:
    -  Running on Bare Metal 
    -  Running on a Cloud Instance
    -  Running as a Kubernetes managed Deployment  
- Proxy/Load Balance traffic to: 
    - A Kubernetes ClusterIP Address and ServicePort
    - A Kubernetes Pod IP Address and Port
    - A Kubernetes worker host's IP Address and Node Port
        - Including cloud instances private or public IP Address

Note that all of the NGINX configuration is managed by LBEX.  The only requirement is that NGINX be installed and executable on the host operating system that LBEX will run on. The LBEX NGINX instance cannot be used for any other purpose.  LBEX writes, and overwrites, all of NGINX's configuration files repeatedly during normal operation.  As such it doesn't play well with any other configuration management system or human operators. 

### Discussion
A deployment of LBEX requires, at a minimum, network connectivity to both the Kubernetes API server and at least one destination address subnet.  The API server can be accessed via `kubectl proxy` for development, but this is not recommended for production deployments.  For normal operation, the standard access via the users existing kubeconfig or the API Server's endpoint is supported.  Network access must be available to at least one destination address space, either the ClusterIP Service IP address space, the Pod IP address space, or the host worker nodes IP address space (public or private).

The LBEX application can run in any environment where some reasonable combination of access to these two resources is available.    
## Using LBEX Example
The following is an example of deploying a Kubernetes service that uses LBEX for it external load balancer.  Assume that our cluster provides an [NTP servers](https://en.wikipedia.org/wiki/Network_Time_Protocol) as a Kubernetes [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/).  The example show here is actually more verbose than necessary, but that's entirely for illustration.  We'll revisit this example after a full discussion of Annotations. The following [Service](https://kubernetes.io/docs/concepts/services-networking/service/) Specification would configure LBEX for the NTP Service. 
```
apiVersion: v1
kind: Service
metadata:
  name: cluster-local-ntp
  labels:
    name: ntp-service
    app: ntp
    version: 1.0.0
  annotations:
    kubernetes.io/loadbalancer-class: loadbalancer-lbex
    loadbalancer.lbex/algorithm: round_robin
    loadbalancer.lbex/upstream-type: node
    loadbalancer.lbex/node-set: host
    loadbalancer.lbex/node-address-type: internal
spec:
  type: NodePort
  selector: 
    app: ntp-pod
  ports:
  - name: ntp-listener
    protocol: UDP
    port: 123
    nodePort: 30123
```

### How It Works
Everything should look very familiar with one obvious exception.  There is nothing out of the ordinary here with respect to the service specification, aside from the metadata object's annotations.  The annotations shown are primarily for illustration purposes (we'll discuss annotations in more detail next).  The annotations show here have the effect of defining:
- an NGINX load balancer that accepts incoming traffic on UDP port 123
- distributes network traffic, round robin, to all Pods running the NTP service
- network traffic is delivered to the worker node's UDP node port 30123

An important consideration is that the LBEX is supplemental to any other load balancers currently in existence in the cluster.  Significantly, this in no way affects the native Kubernetes `kube-proxy` based `iptables` load balancing.  Less obvious may be the fact that any other load balancer defined for a service can operate in parallel with very limited restrictions.  As a side note, it is very likely that a significant portion of the NGINX server configuration directives will eventually become available as ConfigMaps.  

## Annotations
Kubernetes [Annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) currently play a central role in defining how LBEX is used.  Ideally this configuration data will be migrated to Kubernetes [ConfigMaps](https://kubernetes.io/docs/user-guide/configmap/) soon.  When, and if, that happens support will be provided for all existing annotations for several subsequent versions.
### Annotation Definitions 
The annotations defined for LBEX are as follows:
<table border="1">
    <tr>
        <th>Annotations</th>
        <th>Values</th>
        <th>Default</th>
        <th>Required</th>
    </tr>
        <td>kubernetes.io/loadbalancer-class</td>
        <td>loadbalancer-lbex</td>
        <td>None</td>
        <td>True</td>
    </tr>
    <tr>
        <td>loadbalancer.lbex/algorithm</td>
        <td>round_robin, <br />least_conn, <br />least_time<sup>[1]</sup></td>
        <td>round_robin</td>
        <td>False</td>
    </tr>
    <tr>
        <td>loadbalancer.lbex/method<sup>[1]</sup></td>
        <td> connect, <br />first_byte, <br />last_byte, <br />connect inflight, <br />first_byte inflight, <br />last_byte inflight</td>
        <td>connect</td>
        <td>False</td>
    </tr>
    <tr>
        <td>loadbalancer.lbex/resolver</td>
        <td>Any IP Address</td>
        <td>None</td>
        <td>False</td>
    <tr>
    </tr>
        <td>loadbalancer.lbex/upstream-type</td>
        <td>node, <br />pod, <br />cluster-ip</td>
        <td>node</td>
        <td>False</td>
    </tr>
    <tr>
        <td>loadbalancer.lbex/node-set</td>
        <td>host, all</td>
        <td>host</td>
        <td>False</td>
    </tr>
    <tr>
        <td>loadbalancer.lbex/node-address-type</td>
        <td>internal, <br />external</td>
        <td>internal</td>
        <td>False</td>
    </tr>
</table>
    [1] The least_time load balancing method is only available in NGINX Plus

### Annotation Descriptions 
The only mandatory value that must be present for LBEX to serve traffic for the intended Kubernetes Service is `kubernetes.io/loadbalancer-class`.  Every other annotation has either a sensible default or is optional.

<b>loadbalancer.lbex/algorithm</b> - Defaults to round robin, but can also be set to least connections.  The option to select leas time (lowest measured time) is supported, but can only be used with NGINX Plus.

<b>loadbalancer.lbex/method</b> Method is a supplemental argument to the least_time directive.  Similarly, it is supported in LBEX but requires NGINX Plus to function.  See reference: [least_time](http://nginx.org/en/docs/stream/ngx_stream_upstream_module.html#least_time).

<b>loadbalancer.lbex/resolver</b> - Configures name servers used to resolve names of upstream servers into addresses. See reference: [resolver](https://nginx.org/en/docs/stream/ngx_stream_core_module.html#resolver).

<b>loadbalancer.lbex/upstream-type</b> - Upstream type indicates the type of the backend service addresses to direct to.  The default, `node`, directs load balanced traffic to the Kubernetes host worker node and node port.  Alternatively, `pod` directs traffic to the Kubernetes Pod and its' corresponding port.  Finally, `cluster-ip' directs traffic to the Kubernetes Service's ClusterIP.

This final two annotations are only read if, and only if, `loadbalancer.lbex/upstream-type=node`. 
<b>loadbalancer.lbex/node-set</b> - Selects the set of Kubernetes host worker nodes to add to the upstream for the load balancer.  The default `host` ensures that traffic is only directed to noes that are actively running a copy of the services backend pod.  By contrast, `all` will direct traffic to any available Kubernetes worker node.

<b>loadbalancer.lbex/node-address-type</b> - Determines whether to direct load balanced traffic to the node's `internal` private IP address (default), or its' `external` public IP address. 

### Annotation Selection
It is incumbent on the service designer to make sensible selections for annotation values.  For example, it makes no sense to select a node address type of `external` if the worker nodes in the Kubernetes cluster haven't been created with one.  It would also be off to try to select an upstream type of `cluster-ip` if 1) the service doesn't provide one, 2) LBEX is not running as a Pod in the cluster.  By definition a cluster IP address is only accessible to members of the cluster.

## Using LBEX Example - Revisited
Returning to the pervious example, here is the updated version that takes advantage of the default values for all but the one required annotation.  As before, the following [Service Specification](https://kubernetes.io/docs/concepts/services-networking/service/) would configure LBEX for the NTP Service. 
```
apiVersion: v1
kind: Service
metadata:
  name: cluster-local-ntp
  labels:
    name: ntp-service
    app: ntp
    version: 1.0.0
  annotations:
    kubernetes.io/loadbalancer-class: loadbalancer-lbex
 spec:
  type: NodePort
  selector: 
    app: ntp-pod
  ports:
  - name: ntp-listener
    protocol: UDP
    port: 123
    nodePort: 30123
```

So, by taking advantage of several sensible defaults, the Service definition is exactly as it would be were it not using LBEX aside from the addition of a one line annotation.

## Motivation
A very specific use case arises for Google Container Engine (GKE) base Kubernetes services that require an external load balancer, but not a public IP address.  That is, services that need to be exposed to RFC1918 address spaces, but that address space is neither part of the Cluster's IP address space, or the [GCP Subnet Network](https://cloud.google.com/compute/docs/networking#subnet_network) Auto [IP Ranges](https://cloud.google.com/compute/docs/networking#ip_ranges).  This is particularly challenging when connecting to GCP via [Google Cloud VPN](https://cloud.google.com/compute/docs/vpn/overview), where the on premise peer network side of the VPN is also an RFC1918 10/8 network space.  This configuration, in and of itself, presents certain challenges described here: [GCI IP Tables Configuration](https://github.com/samsung-cnct/gci-iptables-conf-agent).   Once the two networks are interconnected, there is still the issue of communicating with the GCP region's private IP subnet range, and further being able to reach exposed Kubernetes services in the Kubernetes Cluster CIDR range.

There were several attempts at solving this problem with a combination of various [Google Cloud Load Balancing](https://cloud.google.com/load-balancing/) components, including using the [GCP Internal Load Balancer](https://cloud.google.com/compute/docs/load-balancing/internal/) and following the model provided by the [Internal Load Balancing using HAProxy on Google Compute Engine](https://cloud.google.com/solutions/internal-load-balancing-haproxy) example.

In the end, the best solution we were able to arrive at was 1) not dynamic and 2) exposed a high order ephemeral port.  
1. This meant that, since the GCP Internal LB solution had to have stable endpoints, there was an external requirement to ensure that service specifications confirmed to certain constraints.  Conversely, anytime a service configuration change was made, or a new service introduce in to the environment, a corresponding LB had to created and/or updated.
2. This was first and foremost unsightly and awkward to manage.  Over time it was the leaky abstraction that was the most bothersome and provided extra motivation to move forward with a better solution.
 
Finally, there are challenges to automating all these things as well.  None of them are insurmountable by any means, but when justifying the engineering effort to automate operations you prefer it to be for the right solution.  


## NGINX Prerequisites

For TCP and UDP load balancing to work, the NGINX image must be built with the `--with-stream` configuration flag to load/enable the required stream processing modules.  In most cases the [NGINX Official Repository](https://hub.docker.com/_/nginx/) 'latest' tagged image will include the stream modules by default.  The easiest way to be certain that the modules are included is to dump the configuration and check for their presence.

For example, running the following command against the `nginx:latest` image shows the following (line breaks added for clarity)

    $ docker run -t nginx:latest nginx -V
    nginx version: nginx/1.11.10
    built by gcc 4.9.2 (Debian 4.9.2-10) 
    built with OpenSSL 1.0.1t  3 May 2016
    TLS SNI support enabled
    configure arguments: --prefix=/etc/nginx 
                --sbin-path=/usr/sbin/nginx 
                --modules-path=/usr/lib/nginx/modules 
                --conf-path=/etc/nginx/nginx.conf 
                --error-log-path=/var/log/nginx/error.log 
                --http-log-path=/var/log/nginx/access.log 
                --pid-path=/var/run/nginx.pid 
                --lock-path=/var/run/nginx.lock 
                --http-client-body-temp-path=/var/cache/nginx/client_temp 
                --http-proxy-temp-path=/var/cache/nginx/proxy_temp 
                --http-fastcgi-temp-path=/var/cache/nginx/fastcgi_temp 
                --http-uwsgi-temp-path=/var/cache/nginx/uwsgi_temp 
                --http-scgi-temp-path=/var/cache/nginx/scgi_temp 
                --user=nginx 
                --group=nginx 
                --with-compat 
                --with-file-aio 
                --with-threads 
                --with-http_addition_module 
                --with-http_auth_request_module 
                --with-http_dav_module 
                --with-http_flv_module 
                --with-http_gunzip_module 
                --with-http_gzip_static_module 
                --with-http_mp4_module 
                --with-http_random_index_module 
                --with-http_realip_module 
                --with-http_secure_link_module 
                --with-http_slice_module 
                --with-http_ssl_module 
                --with-http_stub_status_module 
                --with-http_sub_module 
                --with-http_v2_module 
                --with-mail 
                --with-mail_ssl_module 
                --with-stream 
                --with-stream_realip_module 
                --with-stream_ssl_module 
                --with-stream_ssl_preread_module 
                --with-cc-opt='-g -O2 -fstack-protector-strong -Wformat -Werror=format-security -Wp,-D_FORTIFY_SOURCE=2 -fPIC' 
                --with-ld-opt='-Wl,-z,relro -Wl,-z,now -Wl,
                --as-needed -pie'

As you can see several stream modules are included in the NGINX build configuration. 
