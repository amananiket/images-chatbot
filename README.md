# images-chatbot

This chatbot is written as a simple Go service. It performs 2 tasks - save images and search images. It can understand and parse natural language to perform these tasks


### Dependencies

This project needs https://github.com/amananiket/word2vec-api to be up and running on port 1234. 

### Compile and run

`go run images.go`


### API 

- There's a single endpoint POST `/callback` that is exposed.



