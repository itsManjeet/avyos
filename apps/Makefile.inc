.TARGET 		?= ${.SYSROOTDIR}/usr/apps/$(notdir ${.RELDIR})/exec
EXTRA_TARGETS	+= icon.svg manifest.json
POST_TARGETS	+= $(addprefix ${.SYSROOTDIR}/usr/apps/$(notdir ${.RELDIR})/,${EXTRA_TARGETS})


${.SYSROOTDIR}/usr/apps/$(notdir ${.RELDIR})/%: ${.CURDIR}/%
	@mkdir -p $(dir $@)
	cp -a $< $@
