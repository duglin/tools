kubectl run -ti loader$RANDOM --generator=run-pod/v1 --image duglin/loader --restart=Never --rm=true -- -c 100 -d 1 -t 20 -p=false http://echo.1d2e7332-4a73.us-south.knative.appdomain.cloud
