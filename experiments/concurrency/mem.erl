-module(mem).
-export([main/0]).
main() ->
    K = 100000,
    erlang:garbage_collect(),
    A = erlang:memory(processes),
    Ps = [spawn(fun() -> receive stop -> ok end end) || _ <- lists:seq(1, K)],
    timer:sleep(200),
    B = erlang:memory(processes),
    io:format("erlang native: ~p bytes per idle process~n", [(B - A) div K]),
    [P ! stop || P <- Ps],
    halt().
