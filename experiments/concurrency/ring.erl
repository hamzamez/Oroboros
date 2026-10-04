%% The ring on BEAM, the reference: N processes, T tokens, M rounds each.
%% erlc ring.erl && erl -noshell -s ring main N T M
-module(ring).
-export([main/1]).

relay(Next) ->
    receive V -> Next ! V, relay(Next) end.

head(Next, Live, Done) ->
    receive
        1 when Live =:= 1 -> Done ! done;
        1 -> head(Next, Live - 1, Done);
        V -> Next ! V - 1, head(Next, Live, Done)
    end.

run(N, T, M) ->
    Self = self(),
    H = spawn(fun() -> receive {next, Nx} -> head(Nx, T, Self) end end),
    Last = lists:foldl(fun(_, Nx) -> spawn(fun() -> relay(Nx) end) end, H, lists:seq(1, N - 1)),
    Start = erlang:monotonic_time(nanosecond),
    H ! {next, Last},
    [Last ! M || _ <- lists:seq(1, T)],
    receive done -> ok end,
    El = erlang:monotonic_time(nanosecond) - Start,
    El.

main([NA, TA, MA]) ->
    [N, T, M] = [list_to_integer(atom_to_list(X)) || X <- [NA, TA, MA]],
    run(N, T, M div 10),
    Ds = lists:sort([run(N, T, M) / (N * T * M) || _ <- lists:seq(1, 7)]),
    io:format("erlang native N=~p T=~p M=~p: ~.1f ns/hop (min ~.1f max ~.1f)~n",
              [N, T, M, lists:nth(4, Ds), hd(Ds), lists:last(Ds)]),
    halt().
