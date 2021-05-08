|foo1|bar1    |
|----|--------|
|foo2|baあああ|

|いいい いい|bar1    |
|-----------|--------|
|foo2       |baあああ|

|いいい いい|bar1|
|-----------|----|
|foo2       |    |

|Request|Handled in|Wait time|Actual response time|
|-------|----------|---------|--------------------|
|/      |4s        |0        |4s                  |
|/ping  |4s        |4s       |8s                  |
|/dfd   |5s        |8s       |13s                 |
|/foot  |3s        |5        |2s                  |


