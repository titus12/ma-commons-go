/*
 stsmq: 称之为 server to server message queue （服务器对服务器消息队列），在游戏服务器之间进行通讯时我们可以采用
 gRpc，也可以采用消息队列的形式，我这里会定义对于同级服务我们采用stsmq,对于次级服务采用gRpc，而这个包只负责stsmq的
 实现模块，并非应用模块，他能被其他任何模块使用，这里我们原意准备实现
      redis
      rabbit
      rocket
      kafka
 目前只实现了用redis充当队列的实现，SER_MQ的环境变量可以定义我们采用哪种实运行
 默认我们采用的是redis实现，种队列实现的具体值如下
 SER_MQ = redis         --redis实现
 SER_MQ = kafka         --kafka实现
 SER_MQ = rabbit        --rabbit实现
 SER_MQ = rocket        --rocket实现

注意在实始化的时候 redis的配置是写死的，这里需要改进
*/
package stsmq
