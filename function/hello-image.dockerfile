FROM python:3.9-alpine
COPY hello.py /hello.py
RUN chmod +x /hello.py
CMD [ "python3","/hello.py" ]