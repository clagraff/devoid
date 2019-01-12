


Working on the server/server.go

stuck on managing subscribers to notifications:
What happens if a client subscibes to Position<1,2> because their Entity is there,
and then a mutator _moves_ their entity to Position<4,5>?

They no longer have a reason to be subscribed to Position<1,2>. In fact it may be
a security/visibility issue if a client remains subscribed to notifications they
should not be receiving.

So somehow we need to know how to change subscriptions based on mutations for
subscribers. Maybe. 

In this case, Position<1,2> may still need to be subscibed to if it is still visible
by the client/entity.

So basically, the following would need to happen:


Move intent ->
    MoveTo notification ->
        recalculate source entity sighline subscriptions
    MoveFrom notification ->
        ??? profit ???


Maybe just like we have Intents and Mutators we need Subscriptions or something.
Structs whose purpose is to recalculate which subscriptions needed to be added/removed/updated.
