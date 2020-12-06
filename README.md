# sosreport-operator

To test run:
~~~
make generate
make manifests
make install
make run ENABLE_WEBHOOKS=false
~~~

Test resources are in:
~~~
kubectl apply -f ./config/samples/support_v1alpha1_sosreport1.yaml
kubectl apply -f ./config/samples/support_v1alpha1_sosreport2.yaml
~~~
