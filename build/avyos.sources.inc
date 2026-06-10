fetch:
	@mkdir -p ${SOURCESDIR}
	@for uri in ${SOURCES} ; do \
		if [ ! -e ${SOURCESDIR}/$$(basename $$uri) ] ; then \
			echo "  WGET $$(basename $$uri)"; \
			wget -nc -O ${SOURCESDIR}/$$(basename $$uri).partial $$uri && \
				mv ${SOURCESDIR}/$$(basename $$uri).partial \
					${SOURCESDIR}/$$(basename $$uri) || \
				exit 1; \
		fi; \
		if [ ! -e ${SOURCESDIR}/.extracted-$$(basename $$uri) ] ; then \
			echo "  TAR  $$(basename $$uri)"; \
			tar -xf ${SOURCESDIR}/$$(basename $$uri) -C ${SOURCESDIR} && \
				touch ${SOURCESDIR}/.extracted-$$(basename $$uri) || \
				exit 1; \
		fi; \
	done
