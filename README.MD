# GoHttpRequestTaskProxy

GoHttpRequestTaskProxy is a service written in Go that redirects requests from the user to a third party service sent to the server through a post request to the /TASK/ endpoint in a json format.

<b>This service is made purely for educational purposes and isn't ready for production. </b>

The service uses a postgres database to store the tasks.

# Routing

POST   /task/              :  create a task, returns ID

GET    /task/<taskid>      :  returns a single task by ID

GET    /task/              :  returns all tasks

DELETE /task/<taskid>      :  delete a task by ID

DELETE /task/			   :  delete all tasks


# Roadmap

- [X] Implement basic prototype
- [X] Add docker container
- [ ] Implement the database
- [ ] Port code to work with the database instead of an array
- [ ] Add unit tests
- [ ] Add get request parameterization
