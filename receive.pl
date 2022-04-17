% this is embedded by the library and used for processing Prolog-format results
% see: prolog.go

success(ID, Terms, Projection, Time, More) :-
	'$pengine_success'(ID, Terms, Projection, Time, More).

failure(ID, Time) :-
	'$pengine_failure'(ID, Time).

error(ID, Term) :-
	'$pengine_error'(ID, Term).

create(ID, Data) :-
	( member(slave_limit(Limit), Data) -> true
	; Limit = 0
	),
	!,
	'$pengine_create'(ID, Limit),
	( member(answer(Goal), Data) -> call(Goal)
	; true
	),
	!.

destroy(ID, Result) :-
	call(Result),
	'$pengine_destroy'(ID).

output(ID, Term) :-
	'$pengine_output'(ID, Term).

% prompt(ID, Term) :-
%     false.
