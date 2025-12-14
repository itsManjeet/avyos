fetch-sources:
	@mkdir -p ${SOURCES_PATH}
	@for uri in ${SOURCE_URI} ; do \
		if [ ! -e ${SOURCES_PATH}/$$(basename $$uri) ] ; then \
			echo "  CURL $$(basename $$uri)"; \
			curl -L -C - -O ${SOURCES_PATH}/$$(basename $$uri).partial && \
				mv ${SOURCES_PATH}/$$(basename $$uri).partial \
					${SOURCES_PATH}/$$(basename $$uri) || \
				exit 1; \
		fi; \
		if [ ! -e ${SOURCES_PATH}/.extracted-$$(basename $$uri) ] ; then \
			echo "  TAR  $$(basename $$uri)"; \
			tar -xf ${SOURCES_PATH}/$$(basename $$uri) -C ${SOURCES_PATH} && \
				exit 1; \
		fi; \
	done