// actor的实现包，actor基于mailbox进行实现，我们可以认为每个事物都是一个actor，比如每个用户都是一个独立的actor
// 比如每类型排行榜是一个独立的actor，actor能保证所有消息的处理都是有序，以消息传递作为驱动，而不是以调用方法作
// 为驱动，go程序中搭载actor再加上goroutine可以完美的解决io异步访问,以前在游戏生产过程中去锁化，此包不仅提供了本
// 地化的actor，还提供了网络化的actor，不同机器上的actor以mq为消息通讯媒介，目前只实现了redis的mq传递，但本地化的
// actor则是直接投递的消息.
//
// 注：我们的消息传递都是通过proto来做的，对于一个actor的引用描述实际上是一个proto的结构体 ActorRef
//     但另一个包含处理结构的actor引用描述，我们称之为PID
package actor
