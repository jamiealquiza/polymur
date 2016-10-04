FROM scratch
ADD polymur /
ADD polymur-gateway /
ADD polymur-proxy /
EXPOSE 2300
EXPOSE 443
CMD ["/polymur-gateway"]
