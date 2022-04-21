% example of a minimal pengines server
% try consulting this from swipl
% the tests rely on this by default

:- use_module(library(http/thread_httpd)).
:- use_module(library(http/http_dispatch)).
:- use_module(library(pengines)).

pengines:write_result(prolog, Event, _) :-
    format('Content-type: text/x-prolog; charset=UTF-8~n~n'),
    write_term(Event,
               [ quoted(true),
                 quote_non_ascii(true),            % 🆕
                 character_escapes_unicode(false), % 🆕 must be false or you might see "no solutions found" errors!
                 ignore_ops(true),
                 fullstop(true),
                 blobs(portray),
                 portray_goal(pengines:portray_blob),
                 nl(true)
               ]).

server(Port) :- http_server(http_dispatch, [port(Port)]).

:- server(4242).
