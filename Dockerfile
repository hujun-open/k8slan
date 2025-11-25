# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot
FROM alpine:latest
COPY manager /
COPY ds /
COPY k8slanveth /
COPY --from=quay.io/kubevirt/macvtap-cni:latest /macvtap-cni /
USER 65532:65532
ENTRYPOINT ["/manager"]
