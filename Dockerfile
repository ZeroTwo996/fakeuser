FROM alpine
ADD fakeuser /data/app/
WORKDIR /data/app/
CMD ["/bin/sh", "-c", "./fakeuser"]
