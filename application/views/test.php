<html>

<head>
<title></title>
</head>

<body>


<script src="<?php echo base_url();?>assets/js/highcharts.js"></script>
<script src="<?php echo base_url();?>assets/js/series-label.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/exporting.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/export-data.js"></script>



<figure class="highcharts-figure">
    <div id="test_container"></div>
    
</figure>
	
	

	
<script type="text/javascript">	





	Highcharts.chart('test_container', {
    chart: {
        type: 'spline',
       
    },
    title: {
        text: 'Wind speed during two days',
        align: 'left'
    },
    subtitle: {
        text: '13th & 14th of June, 2022 at two locations in Vik i Sogn, Norway',
        align: 'left'
    },
    xAxis: {
        type: 'datetime',
       
    },
    yAxis: {
        title: {
            text: 'Wind speed (m/s)'
        },
       
  
    },
    tooltip: {
        valueSuffix: ' m/s'
    },
    
    series: [
	{name: 'Extérieur', 		data: [<?php foreach($tempEXT as $row) : ?><?php echo $row->Temp_EXT.','; ?><?php endforeach; ?>] },
	{name: 'Chambres', 			data: [<?php foreach($tempHK1 as $row) : ?><?php echo $row->HK1_Temp_Ambiante.','; ?><?php endforeach; ?>] }, 
	{name: 'Pièce de vie',  	data: [<?php foreach($tempHK2 as $row) : ?><?php echo $row->HK2_Temp_Ambiante.','; ?><?php endforeach; ?>] }, 
	{name: 'E.C.S',		        data: [<?php foreach($tempWW1 as $row) : ?><?php echo $row->WW1_Temp_ECS.','; ?><?php endforeach; ?>]      }
			]
    
});


</script>



</body>
</html>

