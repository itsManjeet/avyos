SUBDIR_MAKEFLAGS := 
SUBDIR_TARGETS	 :=	all compile_db

${SUBDIR_TARGETS}:
	@for dir in ${SUBDIR}; do \
		${MAKE} ${SUBDIR_MAKEFLAGS} -C $$dir $@ || exit 1; \
	done

${SUBDIR}:
	${MAKE} ${SUBDIR_MAKEFLAGS} -C $@

.PHONY: ${SUBDIR_TARGETS} ${SUBDIR}