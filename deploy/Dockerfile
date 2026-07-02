FROM scratch

COPY bin/vmlens-agent /vmlens-agent
COPY config/vmlens.example.yaml /etc/vmlens/vmlens.yaml

EXPOSE 9109
ENTRYPOINT ["/vmlens-agent"]
CMD ["--config", "/etc/vmlens/vmlens.yaml", "--demo", "--listen", ":9109"]
